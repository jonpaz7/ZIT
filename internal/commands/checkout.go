package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"zit/internal/object"
	"zit/internal/repo"
)

func Checkout(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("checkout requires a branch or commit hash")
	}

	target := args[0]
	var sparsePaths []string

	// Check if "--" separator is provided
	dashIdx := -1
	for i, a := range args {
		if a == "--" {
			dashIdx = i
			break
		}
	}
	if dashIdx != -1 {
		target = args[0]
		if dashIdx > 1 {
			// e.g. zit checkout main -- path1 path2
			target = args[1]
		}
		if dashIdx == 0 && len(args) > 1 {
			target = "" // only paths given, keep current HEAD
			sparsePaths = args[1:]
		} else {
			sparsePaths = args[dashIdx+1:]
		}
	}

	r, err := repo.Find(".")
	if err != nil {
		return err
	}

	targetCommit := ""
	targetRef := ""

	if target == "" {
		// Just sparse checkout on current commit
		targetCommit, err = r.ResolveHead()
		if err != nil || targetCommit == "" {
			return fmt.Errorf("no current commit to checkout")
		}
	} else {
		// Check if target is a branch
		branchRef := "refs/heads/" + target
		commitAtBranch, _ := r.GetRef(branchRef)
		if commitAtBranch != "" {
			targetRef = branchRef
			targetCommit = commitAtBranch
		} else {
			// Direct commit hash
			targetCommit = target
		}
	}

	// 1. Fetch commit object
	rawCommit, err := r.Store().Get(targetCommit)
	if err != nil {
		return fmt.Errorf("failed to get commit %s: %w", targetCommit, err)
	}
	cObj, err := object.ParseRaw(rawCommit)
	if err != nil {
		return fmt.Errorf("parse commit %s: %w", targetCommit, err)
	}
	commit, err := object.ParseCommit(cObj.Data)
	if err != nil {
		return fmt.Errorf("parse commit details %s: %w", targetCommit, err)
	}

	// 2. Recursively fetch trees and collect all file paths
	allFiles, err := object.FlattenTree(commit.Tree, r.Store())
	if err != nil {
		return fmt.Errorf("flatten commit tree: %w", err)
	}

	// 3. Determine which files should be downloaded
	type fileTask struct {
		relPath  string
		blobHash string
	}
	var toDownload []fileTask
	for relPath, blobHash := range allFiles {
		if len(sparsePaths) == 0 || isPathInSparse(relPath, sparsePaths) {
			toDownload = append(toDownload, fileTask{
				relPath:  relPath,
				blobHash: blobHash,
			})
		}
	}

	// 4. Clean up old files on disk that are not in the new checkout
	oldIndex, _ := r.GetIndex()
	for oldPath := range oldIndex {
		shouldExist := false
		if _, ok := allFiles[oldPath]; ok {
			if len(sparsePaths) == 0 || isPathInSparse(oldPath, sparsePaths) {
				shouldExist = true
			}
		}
		if !shouldExist {
			fullPath := filepath.Join(r.WorkDir, filepath.FromSlash(oldPath))
			_ = os.Remove(fullPath)
			// Remove empty parent dirs
			dir := filepath.Dir(fullPath)
			for dir != r.WorkDir && dir != "." {
				if err := os.Remove(dir); err != nil {
					break
				}
				dir = filepath.Dir(dir)
			}
		}
	}

	// 5. Parallel download blobs using worker pool (8 goroutines)
	const numWorkers = 8
	tasksChan := make(chan fileTask, len(toDownload))
	for _, t := range toDownload {
		tasksChan <- t
	}
	close(tasksChan)

	var wg sync.WaitGroup
	errChan := make(chan error, len(toDownload))

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasksChan {
				blobRaw, err := r.Store().Get(task.blobHash)
				if err != nil {
					errChan <- fmt.Errorf("download blob %s for %s: %w", task.blobHash, task.relPath, err)
					return
				}
				blobObj, err := object.ParseRaw(blobRaw)
				if err != nil {
					errChan <- fmt.Errorf("parse blob %s: %w", task.blobHash, err)
					return
				}

				dest := filepath.Join(r.WorkDir, filepath.FromSlash(task.relPath))
				if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
					errChan <- err
					return
				}
				if err := os.WriteFile(dest, blobObj.Data, 0644); err != nil {
					errChan <- err
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errChan)
	if len(errChan) > 0 {
		return <-errChan
	}

	// 6. Update sparse.json, index.json, and HEAD
	if err := r.SaveSparse(sparsePaths); err != nil {
		return fmt.Errorf("save sparse: %w", err)
	}
	if err := r.SaveIndex(allFiles); err != nil {
		return fmt.Errorf("save index: %w", err)
	}

	if targetRef != "" {
		if err := r.SetHeadRef(targetRef); err != nil {
			return fmt.Errorf("set head ref: %w", err)
		}
		branchName := strings.TrimPrefix(targetRef, "refs/heads/")
		fmt.Printf("Switched to branch '%s'\n", branchName)
	} else if target != "" {
		if err := r.SetHeadCommit(targetCommit); err != nil {
			return fmt.Errorf("set head commit: %w", err)
		}
		short := targetCommit
		if len(short) > 7 {
			short = short[:7]
		}
		fmt.Printf("HEAD is now at %s\n", short)
	}

	if len(sparsePaths) > 0 {
		fmt.Printf("Sparse checkout updated: %d files downloaded for [%s]\n", len(toDownload), strings.Join(sparsePaths, ", "))
	} else {
		fmt.Printf("Checked out %d files\n", len(toDownload))
	}

	return nil
}
