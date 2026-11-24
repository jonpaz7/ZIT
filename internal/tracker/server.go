package tracker

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"zit/internal/crypto"
)

const PeerTTL = 10 * time.Minute

type RepoState struct {
	PublicKey string              `json:"public_key"`
	Refs      map[string]*RefData `json:"refs"`
}

type TrackerServer struct {
	mu        sync.RWMutex
	providers map[string]map[string]time.Time // hash -> peer -> lastSeen
	repos     map[string]*RepoState           // repoId -> RepoState
	dumpFile  string
}

func NewServer(dumpFile string) *TrackerServer {
	s := &TrackerServer{
		providers: make(map[string]map[string]time.Time),
		repos:     make(map[string]*RepoState),
		dumpFile:  dumpFile,
	}
	if dumpFile != "" {
		_ = s.loadDump()
	}
	go s.gcLoop()
	return s
}

func (s *TrackerServer) gcLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		s.cleanupExpired()
	}
}

func (s *TrackerServer) cleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for hash, peers := range s.providers {
		for peer, lastSeen := range peers {
			if now.Sub(lastSeen) > PeerTTL {
				delete(peers, peer)
			}
		}
		if len(peers) == 0 {
			delete(s.providers, hash)
		}
	}
}

func (s *TrackerServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/announce", s.handleAnnounce)
	mux.HandleFunc("/providers/", s.handleProviders)
	mux.HandleFunc("/refs/", s.handleRefs)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	return mux
}

func (s *TrackerServer) handleAnnounce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req AnnounceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Peer == "" || len(req.Hashes) == 0 {
		http.Error(w, "missing peer or hashes", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	now := time.Now()
	for _, h := range req.Hashes {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if _, ok := s.providers[h]; !ok {
			s.providers[h] = make(map[string]time.Time)
		}
		s.providers[h][req.Peer] = now
	}
	s.mu.Unlock()

	s.saveDump()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AnnounceResponse{
		Status: "ok",
		Count:  len(req.Hashes),
	})
}

func (s *TrackerServer) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hash := strings.TrimPrefix(r.URL.Path, "/providers/")
	hash = strings.TrimSpace(hash)
	if hash == "" {
		http.Error(w, "missing hash", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	peersMap, ok := s.providers[hash]
	var activePeers []string
	if ok {
		now := time.Now()
		for peer, lastSeen := range peersMap {
			if now.Sub(lastSeen) <= PeerTTL {
				activePeers = append(activePeers, peer)
			}
		}
	}
	s.mu.RUnlock()

	if activePeers == nil {
		activePeers = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activePeers)
}

func (s *TrackerServer) handleRefs(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/refs/")
	parts := strings.SplitN(path, "/", 2)
	repoId := parts[0]
	if repoId == "" {
		http.Error(w, "missing repoId", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		repo, ok := s.repos[repoId]
		var refs map[string]*RefData
		if ok {
			refs = repo.Refs
		} else {
			refs = make(map[string]*RefData)
		}
		s.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(refs)

	case http.MethodPut:
		if len(parts) < 2 || parts[1] == "" {
			http.Error(w, "missing ref name in URL", http.StatusBadRequest)
			return
		}
		refName := parts[1] // e.g. "heads/main" or "refs/heads/main"
		if !strings.HasPrefix(refName, "refs/") {
			refName = "refs/" + refName
		}

		var update RefData
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}

		s.mu.Lock()

		repo, exists := s.repos[repoId]
		pubKeyHex := update.PublicKey
		if !exists {
			if pubKeyHex == "" {
				s.mu.Unlock()
				http.Error(w, "repo does not exist yet; must supply public_key", http.StatusBadRequest)
				return
			}
			pub, err := crypto.DecodePublicKey(pubKeyHex)
			if err != nil {
				s.mu.Unlock()
				http.Error(w, "invalid public_key: "+err.Error(), http.StatusBadRequest)
				return
			}
			if crypto.RepoID(pub) != repoId {
				s.mu.Unlock()
				http.Error(w, "public key does not match repoId", http.StatusForbidden)
				return
			}
			repo = &RepoState{
				PublicKey: pubKeyHex,
				Refs:      make(map[string]*RefData),
			}
			s.repos[repoId] = repo
		} else {
			if pubKeyHex == "" {
				pubKeyHex = repo.PublicKey
			} else if pubKeyHex != repo.PublicKey {
				s.mu.Unlock()
				http.Error(w, "public key mismatch with registered repo", http.StatusForbidden)
				return
			}
		}

		pub, err := crypto.DecodePublicKey(repo.PublicKey)
		if err != nil {
			s.mu.Unlock()
			http.Error(w, "corrupt repo public key: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Verify signature: message is repoId + ":" + refName + ":" + update.Commit
		msg := []byte(fmt.Sprintf("%s:%s:%s", repoId, refName, update.Commit))
		if !crypto.Verify(pub, msg, update.Signature) {
			// also try verifying commit directly in case signed that way
			if !crypto.Verify(pub, []byte(update.Commit), update.Signature) {
				s.mu.Unlock()
				http.Error(w, "signature verification failed", http.StatusUnauthorized)
				return
			}
		}

		update.PublicKey = repo.PublicKey
		update.UpdatedAt = time.Now()
		repo.Refs[refName] = &update

		s.mu.Unlock()

		s.saveDump()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "ref": refName, "commit": update.Commit})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type dumpPayload struct {
	Providers map[string]map[string]time.Time `json:"providers"`
	Repos     map[string]*RepoState           `json:"repos"`
}

func (s *TrackerServer) saveDump() {
	if s.dumpFile == "" {
		return
	}
	s.mu.RLock()
	payload := dumpPayload{
		Providers: s.providers,
		Repos:     s.repos,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	s.mu.RUnlock()
	if err == nil {
		_ = os.WriteFile(s.dumpFile, data, 0644)
	}
}

func (s *TrackerServer) loadDump() error {
	data, err := os.ReadFile(s.dumpFile)
	if err != nil {
		return err
	}
	var payload dumpPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if payload.Providers != nil {
		s.providers = payload.Providers
	}
	if payload.Repos != nil {
		s.repos = payload.Repos
	}
	log.Printf("Loaded tracker state from %s", s.dumpFile)
	return nil
}
