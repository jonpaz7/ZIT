package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"zit/internal/crypto"
	"zit/internal/object"
	"zit/internal/repo"
	"zit/internal/tracker"
)

func Clone(trackerURL, repoID, destDir, peerAddr string) error {
	if trackerURL == "" || repoID == "" {
		return fmt.Errorf("usage: zit clone <tracker-url> <repoId> [dir]")
	}

	if destDir == "" {
		destDir = repoID
		if len(destDir) > 8 {
			destDir = destDir[:8]
		}
	}

	absDest, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}

	if fi, err := os.Stat(absDest); err == nil && fi.IsDir() {
		entries, _ := os.ReadDir(absDest)
		if len(entries) > 0 {
			return fmt.Errorf("destination path '%s' already exists and is not an empty directory", destDir)
		}
	}

	fmt.Printf("Cloning repository %s from %s into %s...\n", repoID, trackerURL, destDir)

	if err := os.MkdirAll(absDest, 0755); err != nil {
		return err
	}

	// Create repository skeleton
	zPath := filepath.Join(absDest, repo.ZitDir)
	for _, d := range []string{
		zPath,
		filepath.Join(zPath, repo.ObjectsDir),
		filepath.Join(zPath, repo.RefsDir, "heads"),
		filepath.Join(zPath, repo.KeysDir),
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	// Query tracker for refs
	trackerCli := tracker.NewClient(trackerURL)
	refs, err := trackerCli.GetRefs(repoID)
	if err != nil {
		return fmt.Errorf("fetch refs from tracker: %w", err)
	}
	if len(refs) == 0 {
		return fmt.Errorf("tracker returned no refs for repo %s", repoID)
	}

	// Verify ref signatures and download commits & trees
	var defaultCommit string
	var defaultRefName string

	// Create initial repo struct to use Store
	r := &repo.Repository{
		WorkDir: absDest,
		ZitPath: zPath,
		Config: repo.Config{
			Tracker: trackerURL,
			Peer:    peerAddr,
			RepoID:  repoID,
		},
	}
	if err := r.SaveConfig(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	// Re-load to initialize stores
	r, err = repo.Find(absDest)
	if err != nil {
		return fmt.Errorf("load cloned repo: %w", err)
	}

	treeCount := 0
	commitCount := 0

	for refName, refData := range refs {
		// Verify signature if public key provided
		if refData.PublicKey != "" {
			pub, err := crypto.DecodePublicKey(refData.PublicKey)
			if err == nil {
				if crypto.RepoID(pub) != repoID {
					return fmt.Errorf("corrupt ref %s: public key does not match repoID", refName)
				}
				msg := []byte(fmt.Sprintf("%s:%s:%s", repoID, refName, refData.Commit))
				if !crypto.Verify(pub, msg, refData.Signature) && !crypto.Verify(pub, []byte(refData.Commit), refData.Signature) {
					return fmt.Errorf("security alert: ref %s signature verification failed!", refName)
				}
				// Save verified public key locally
				_ = os.WriteFile(filepath.Join(zPath, repo.KeysDir, "public.key"), []byte(refData.PublicKey+"\n"), 0644)
			}
		}

		// Save ref
		if err := r.SetRef(refName, refData.Commit); err != nil {
			return fmt.Errorf("save ref %s: %w", refName, err)
		}

		// Download commit object
		rawCommit, err := r.Store().Get(refData.Commit)
		if err != nil {
			return fmt.Errorf("download commit %s: %w", refData.Commit, err)
		}
		commitCount++

		cObj, err := object.ParseRaw(rawCommit)
		if err != nil {
			return fmt.Errorf("parse commit %s: %w", refData.Commit, err)
		}
		commit, err := object.ParseCommit(cObj.Data)
		if err != nil {
			return fmt.Errorf("parse commit fields %s: %w", refData.Commit, err)
		}

		// Flatten tree downloads all tree objects (no blobs!)
		allFiles, err := object.FlattenTree(commit.Tree, r.Store())
		if err != nil {
			return fmt.Errorf("download trees for commit %s: %w", refData.Commit, err)
		}
		treeCount++

		if refName == repo.DefaultBranch || defaultCommit == "" {
			defaultCommit = refData.Commit
			defaultRefName = refName
			// Populate index with tree entries from main branch
			_ = r.SaveIndex(allFiles)
		}
	}

	if defaultRefName == "" {
		defaultRefName = repo.DefaultBranch
	}
	_ = r.SetHeadRef(defaultRefName)
	// Initial clone has no blobs on disk, set sparse to prevent status showing all deleted
	_ = r.SaveSparse([]string{})

	fmt.Printf("Cloning completed successfully:\n")
	fmt.Printf("  Commits downloaded: %d\n", commitCount)
	fmt.Printf("  Trees downloaded:   %d\n", treeCount)
	fmt.Printf("  Blobs downloaded:   0 (lazy checkout)\n")
	fmt.Printf("\nRepository is ready in %s.\n", destDir)
	fmt.Printf("Use 'zit checkout <branch> -- <paths...>' or 'zit checkout <branch>' to download files.\n")
	return nil
}
