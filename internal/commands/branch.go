package commands

import (
	"fmt"
	"strings"
	"zit/internal/repo"
)

func Branch(name string) error {
	r, err := repo.Find(".")
	if err != nil {
		return err
	}

	if name == "" {
		branches, current, err := r.ListBranches()
		if err != nil {
			return err
		}
		if len(branches) == 0 {
			if current != "" {
				fmt.Printf("* %s (initial branch, no commits yet)\n", current)
			}
			return nil
		}
		for _, b := range branches {
			prefix := "  "
			if b == current {
				prefix = "* "
			}
			fmt.Printf("%s%s\n", prefix, b)
		}
		return nil
	}

	// Create new branch
	name = strings.TrimSpace(name)
	targetCommit, err := r.ResolveHead()
	if err != nil || targetCommit == "" {
		return fmt.Errorf("cannot create branch '%s': not a valid object name: 'HEAD'", name)
	}

	refName := "refs/heads/" + name
	existing, _ := r.GetRef(refName)
	if existing != "" {
		return fmt.Errorf("a branch named '%s' already exists", name)
	}

	if err := r.SetRef(refName, targetCommit); err != nil {
		return fmt.Errorf("create branch: %w", err)
	}

	return nil
}
