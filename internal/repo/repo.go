package repo

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"zit/internal/crypto"
	"zit/internal/peer"
	"zit/internal/store"
	"zit/internal/tracker"
)

const (
	ZitDir      = ".zit"
	ConfigFile  = "config.json"
	HeadFile    = "HEAD"
	IndexFile   = "index.json"
	SparseFile  = "sparse.json"
	ObjectsDir  = "objects"
	RefsDir     = "refs"
	KeysDir     = "keys"
	DefaultBranch = "refs/heads/main"
)

var (
	ErrNotInRepo = errors.New("not a zit repository (or any of the parent directories)")
)

type Config struct {
	Tracker string `json:"tracker,omitempty"`
	Peer    string `json:"peer,omitempty"`
	RepoID  string `json:"repo_id,omitempty"`
}

type Repository struct {
	WorkDir    string
	ZitPath    string
	Config     Config
	localStore *store.LocalStore
	netStore   *store.NetworkStore
}

// Find locates the .zit directory from current or parent directories.
func Find(startDir string) (*Repository, error) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}
	curr := abs
	for {
		zPath := filepath.Join(curr, ZitDir)
		if fi, err := os.Stat(zPath); err == nil && fi.IsDir() {
			repo := &Repository{
				WorkDir: curr,
				ZitPath: zPath,
			}
			if err := repo.load(); err != nil {
				return nil, err
			}
			return repo, nil
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return nil, ErrNotInRepo
}

// Init creates a new zit repository at workDir.
func Init(workDir, trackerURL, peerAddr string) (*Repository, error) {
	abs, err := filepath.Abs(workDir)
	if err != nil {
		return nil, err
	}
	zPath := filepath.Join(abs, ZitDir)
	if fi, err := os.Stat(zPath); err == nil && fi.IsDir() {
		return nil, fmt.Errorf("zit repository already exists in %s", zPath)
	}

	dirs := []string{
		zPath,
		filepath.Join(zPath, ObjectsDir),
		filepath.Join(zPath, RefsDir, "heads"),
		filepath.Join(zPath, KeysDir),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, err
		}
	}

	pub, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate keypair: %w", err)
	}
	pubHex := crypto.EncodePublicKey(pub)
	privHex := crypto.EncodePrivateKey(priv)
	repoID := crypto.RepoID(pub)

	if err := os.WriteFile(filepath.Join(zPath, KeysDir, "public.key"), []byte(pubHex+"\n"), 0644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(zPath, KeysDir, "private.key"), []byte(privHex+"\n"), 0600); err != nil {
		return nil, err
	}

	cfg := Config{
		Tracker: trackerURL,
		Peer:    peerAddr,
		RepoID:  repoID,
	}
	cfgData, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(filepath.Join(zPath, ConfigFile), cfgData, 0644); err != nil {
		return nil, err
	}

	// HEAD points to default branch refs/heads/main
	headContent := fmt.Sprintf("ref: %s\n", DefaultBranch)
	if err := os.WriteFile(filepath.Join(zPath, HeadFile), []byte(headContent), 0644); err != nil {
		return nil, err
	}

	// Empty staging index
	if err := os.WriteFile(filepath.Join(zPath, IndexFile), []byte("{}\n"), 0644); err != nil {
		return nil, err
	}

	repo := &Repository{
		WorkDir: abs,
		ZitPath: zPath,
		Config:  cfg,
	}
	if err := repo.initStores(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *Repository) load() error {
	cfgFile := filepath.Join(r.ZitPath, ConfigFile)
	if data, err := os.ReadFile(cfgFile); err == nil {
		_ = json.Unmarshal(data, &r.Config)
	}
	return r.initStores()
}

func (r *Repository) initStores() error {
	r.localStore = store.NewLocalStore(filepath.Join(r.ZitPath, ObjectsDir))
	var trackerLookup store.ProviderLookup
	if r.Config.Tracker != "" {
		trackerLookup = tracker.NewClient(r.Config.Tracker)
	}
	r.netStore = store.NewNetworkStore(r.localStore, trackerLookup, peer.NewClient())
	return nil
}

func (r *Repository) LocalStore() *store.LocalStore {
	return r.localStore
}

func (r *Repository) Store() *store.NetworkStore {
	return r.netStore
}

func (r *Repository) SaveConfig() error {
	data, err := json.MarshalIndent(r.Config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.ZitPath, ConfigFile), data, 0644)
}

// GetHead returns the current HEAD target.
// Returns (refName, isBranch, err). E.g. ("refs/heads/main", true, nil) or ("commitHash", false, nil).
func (r *Repository) GetHead() (string, bool, error) {
	data, err := os.ReadFile(filepath.Join(r.ZitPath, HeadFile))
	if err != nil {
		return "", false, err
	}
	str := strings.TrimSpace(string(data))
	if strings.HasPrefix(str, "ref:") {
		ref := strings.TrimSpace(strings.TrimPrefix(str, "ref:"))
		return ref, true, nil
	}
	return str, false, nil
}

func (r *Repository) SetHeadRef(ref string) error {
	if !strings.HasPrefix(ref, "refs/") {
		ref = "refs/heads/" + ref
	}
	content := fmt.Sprintf("ref: %s\n", ref)
	return os.WriteFile(filepath.Join(r.ZitPath, HeadFile), []byte(content), 0644)
}

func (r *Repository) SetHeadCommit(commitHash string) error {
	content := strings.TrimSpace(commitHash) + "\n"
	return os.WriteFile(filepath.Join(r.ZitPath, HeadFile), []byte(content), 0644)
}

// ResolveHead resolves the current commit hash that HEAD points to.
func (r *Repository) ResolveHead() (string, error) {
	ref, isBranch, err := r.GetHead()
	if err != nil {
		return "", err
	}
	if !isBranch {
		return ref, nil
	}
	return r.GetRef(ref)
}

// GetRef returns the commit hash stored at ref (e.g. "refs/heads/main").
func (r *Repository) GetRef(ref string) (string, error) {
	if !strings.HasPrefix(ref, "refs/") {
		ref = "refs/heads/" + ref
	}
	p := filepath.Join(r.ZitPath, filepath.FromSlash(ref))
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // ref not created yet (e.g. unborn branch)
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// SetRef updates the commit hash stored at ref.
func (r *Repository) SetRef(ref, commitHash string) error {
	if !strings.HasPrefix(ref, "refs/") {
		ref = "refs/heads/" + ref
	}
	p := filepath.Join(r.ZitPath, filepath.FromSlash(ref))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(strings.TrimSpace(commitHash)+"\n"), 0644)
}

// ListBranches lists local branch names and returns the current active branch.
func (r *Repository) ListBranches() ([]string, string, error) {
	headsDir := filepath.Join(r.ZitPath, RefsDir, "heads")
	var branches []string
	entries, err := os.ReadDir(headsDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				branches = append(branches, e.Name())
			}
		}
	}
	headRef, isBranch, _ := r.GetHead()
	current := ""
	if isBranch {
		current = strings.TrimPrefix(headRef, "refs/heads/")
	}
	return branches, current, nil
}

// GetIndex loads the staging index map (relPath -> blobHash).
func (r *Repository) GetIndex() (map[string]string, error) {
	p := filepath.Join(r.ZitPath, IndexFile)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}
	idx := make(map[string]string)
	if err := json.Unmarshal(data, &idx); err != nil {
		return make(map[string]string), nil
	}
	return idx, nil
}

// SaveIndex persists the staging index map.
func (r *Repository) SaveIndex(idx map[string]string) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.ZitPath, IndexFile), data, 0644)
}

// GetSparse returns the sparse paths filter list, or empty if non-sparse.
func (r *Repository) GetSparse() ([]string, error) {
	p := filepath.Join(r.ZitPath, SparseFile)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var paths []string
	if err := json.Unmarshal(data, &paths); err != nil {
		return nil, err
	}
	return paths, nil
}

// SaveSparse persists the sparse filter paths.
func (r *Repository) SaveSparse(paths []string) error {
	p := filepath.Join(r.ZitPath, SparseFile)
	if len(paths) == 0 {
		_ = os.Remove(p)
		return nil
	}
	data, err := json.MarshalIndent(paths, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// GetKeys returns public and private keys if available in .zit/keys.
func (r *Repository) GetKeys() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pubPath := filepath.Join(r.ZitPath, KeysDir, "public.key")
	privPath := filepath.Join(r.ZitPath, KeysDir, "private.key")

	pubBytes, err := os.ReadFile(pubPath)
	if err != nil {
		return nil, nil, fmt.Errorf("public key not found: %w", err)
	}
	pub, err := crypto.DecodePublicKey(strings.TrimSpace(string(pubBytes)))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid public key: %w", err)
	}

	privBytes, err := os.ReadFile(privPath)
	if err != nil {
		// private key might be absent in cloned replica
		return pub, nil, nil
	}
	priv, err := crypto.DecodePrivateKey(strings.TrimSpace(string(privBytes)))
	if err != nil {
		return pub, nil, fmt.Errorf("invalid private key: %w", err)
	}
	return pub, priv, nil
}
