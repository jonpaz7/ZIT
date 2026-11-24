package commands

import (
	"fmt"
	"strings"
	"time"
	"zit/internal/object"
	"zit/internal/repo"
)

func Log() error {
	r, err := repo.Find(".")
	if err != nil {
		return err
	}

	currCommitHash, err := r.ResolveHead()
	if err != nil || currCommitHash == "" {
		return fmt.Errorf("current branch has no commits yet")
	}

	for currCommitHash != "" {
		raw, err := r.Store().Get(currCommitHash)
		if err != nil {
			return fmt.Errorf("get commit %s: %w", currCommitHash, err)
		}
		obj, err := object.ParseRaw(raw)
		if err != nil {
			return fmt.Errorf("parse commit %s: %w", currCommitHash, err)
		}
		commit, err := object.ParseCommit(obj.Data)
		if err != nil {
			return fmt.Errorf("parse commit fields %s: %w", currCommitHash, err)
		}

		t := time.Unix(commit.Timestamp, 0)
		fmt.Printf("commit %s\n", currCommitHash)
		fmt.Printf("Author: %s\n", commit.Author)
		fmt.Printf("Date:   %s\n\n", t.Format(time.RFC1123))

		lines := strings.Split(commit.Message, "\n")
		for _, l := range lines {
			fmt.Printf("    %s\n", l)
		}
		fmt.Println()

		currCommitHash = commit.Parent
	}

	return nil
}
