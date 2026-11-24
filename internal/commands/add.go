package commands

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"zit/internal/object"
	"zit/internal/repo"
)

func Add(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("nothing specified, nothing added")
	}

	r, err := repo.Find(".")
	if err != nil {
		return err
	}

	index, err := r.GetIndex()
	if err != nil {
		return fmt.Errorf("read index: %w", err)
	}

	for _, targetPath := range paths {
		absTarget, err := filepath.Abs(targetPath)
		if err != nil {
			return err
		}

		info, err := os.Stat(absTarget)
		if err != nil {
			return fmt.Errorf("cannot add %s: %w", targetPath, err)
		}

		if info.IsDir() {
			err = filepath.WalkDir(absTarget, func(p string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if d.IsDir() {
					if d.Name() == repo.ZitDir {
						return filepath.SkipDir
					}
					return nil
				}
				return addFile(r, p, index)
			})
			if err != nil {
				return err
			}
		} else {
			if err := addFile(r, absTarget, index); err != nil {
				return err
			}
		}
	}

	if err := r.SaveIndex(index); err != nil {
		return fmt.Errorf("save index: %w", err)
	}

	return nil
}

func addFile(r *repo.Repository, absPath string, index map[string]string) error {
	rel, err := filepath.Rel(r.WorkDir, absPath)
	if err != nil {
		return err
	}
	cleanRel := filepath.ToSlash(rel)
	if strings.HasPrefix(cleanRel, repo.ZitDir) || cleanRel == repo.ZitDir {
		return nil
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", cleanRel, err)
	}

	blobObj := object.NewObject(object.TypeBlob, content)
	if _, err := r.LocalStore().Put(blobObj.RawData); err != nil {
		return fmt.Errorf("save blob for %s: %w", cleanRel, err)
	}

	index[cleanRel] = blobObj.Hash
	return nil
}
