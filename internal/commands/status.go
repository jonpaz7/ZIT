package commands

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"zit/internal/object"
	"zit/internal/repo"
)

func Status() error {
	r, err := repo.Find(".")
	if err != nil {
		return err
	}

	headRef, isBranch, err := r.GetHead()
	if err != nil {
		return fmt.Errorf("read HEAD: %w", err)
	}

	if isBranch {
		branchName := strings.TrimPrefix(headRef, "refs/heads/")
		fmt.Printf("On branch %s\n", branchName)
	} else {
		short := headRef
		if len(short) > 7 {
			short = short[:7]
		}
		fmt.Printf("HEAD detached at %s\n", short)
	}

	sparsePaths, _ := r.GetSparse()
	if len(sparsePaths) > 0 {
		fmt.Printf("Sparse checkout active: %s\n", strings.Join(sparsePaths, ", "))
	}

	// 1. Get HEAD commit files map
	headFiles := make(map[string]string)
	headCommitHash, _ := r.ResolveHead()
	if headCommitHash != "" {
		raw, err := r.LocalStore().Get(headCommitHash)
		if err == nil {
			if obj, err := object.ParseRaw(raw); err == nil {
				if c, err := object.ParseCommit(obj.Data); err == nil {
					treeFiles, _ := object.FlattenTree(c.Tree, r.LocalStore())
					if treeFiles != nil {
						headFiles = treeFiles
					}
				}
			}
		}
	}

	// 2. Get Index files map
	index, err := r.GetIndex()
	if err != nil {
		return fmt.Errorf("read index: %w", err)
	}

	// Compare Index vs HEAD (Staged)
	var stagedAdded, stagedModified, stagedDeleted []string
	for path, idxHash := range index {
		headHash, inHead := headFiles[path]
		if !inHead {
			stagedAdded = append(stagedAdded, path)
		} else if headHash != idxHash {
			stagedModified = append(stagedModified, path)
		}
	}
	for path := range headFiles {
		if _, inIndex := index[path]; !inIndex {
			stagedDeleted = append(stagedDeleted, path)
		}
	}

	// Compare Working Directory vs Index (Unstaged & Untracked)
	var unstagedModified, unstagedDeleted []string
	var untracked []string

	// Walk disk files
	diskFiles := make(map[string]bool)
	err = filepath.WalkDir(r.WorkDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == repo.ZitDir {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(r.WorkDir, p)
		if err != nil {
			return nil
		}
		cleanRel := filepath.ToSlash(rel)
		diskFiles[cleanRel] = true

		idxHash, inIndex := index[cleanRel]
		if !inIndex {
			untracked = append(untracked, cleanRel)
		} else {
			// Check if modified on disk
			content, err := os.ReadFile(p)
			if err == nil {
				blobObj := object.NewObject(object.TypeBlob, content)
				if blobObj.Hash != idxHash {
					unstagedModified = append(unstagedModified, cleanRel)
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Check for deleted files (in index, not on disk)
	for path := range index {
		if !diskFiles[path] {
			// If sparse checkout is active and path is outside sparse selection,
			// it was intentionally not downloaded, so not deleted!
			if len(sparsePaths) > 0 && !isPathInSparse(path, sparsePaths) {
				continue
			}
			unstagedDeleted = append(unstagedDeleted, path)
		}
	}

	sort.Strings(stagedAdded)
	sort.Strings(stagedModified)
	sort.Strings(stagedDeleted)
	sort.Strings(unstagedModified)
	sort.Strings(unstagedDeleted)
	sort.Strings(untracked)

	hasStaged := len(stagedAdded)+len(stagedModified)+len(stagedDeleted) > 0
	hasUnstaged := len(unstagedModified)+len(unstagedDeleted) > 0

	if hasStaged {
		fmt.Println("\nChanges to be committed:")
		for _, p := range stagedAdded {
			fmt.Printf("  new file:   %s\n", p)
		}
		for _, p := range stagedModified {
			fmt.Printf("  modified:   %s\n", p)
		}
		for _, p := range stagedDeleted {
			fmt.Printf("  deleted:    %s\n", p)
		}
	}

	if hasUnstaged {
		fmt.Println("\nChanges not staged for commit:")
		for _, p := range unstagedModified {
			fmt.Printf("  modified:   %s\n", p)
		}
		for _, p := range unstagedDeleted {
			fmt.Printf("  deleted:    %s\n", p)
		}
	}

	if len(untracked) > 0 {
		fmt.Println("\nUntracked files:")
		for _, p := range untracked {
			fmt.Printf("  %s\n", p)
		}
	}

	if !hasStaged && !hasUnstaged && len(untracked) == 0 {
		fmt.Println("nothing to commit, working tree clean")
	}

	return nil
}

func isPathInSparse(filePath string, sparsePaths []string) bool {
	cleanFile := filepath.ToSlash(filePath)
	for _, sp := range sparsePaths {
		cleanSp := strings.TrimPrefix(filepath.ToSlash(sp), "./")
		cleanSp = strings.TrimSuffix(cleanSp, "/")
		if cleanSp == "" || cleanSp == "." {
			return true
		}
		if cleanFile == cleanSp || strings.HasPrefix(cleanFile, cleanSp+"/") {
			return true
		}
	}
	return false
}
