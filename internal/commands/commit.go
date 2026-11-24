package commands

import (
	"fmt"
	"os"
	"strings"
	"time"
	"zit/internal/object"
	"zit/internal/repo"
)

func Commit(message, author string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return fmt.Errorf("commit message cannot be empty")
	}

	r, err := repo.Find(".")
	if err != nil {
		return err
	}

	index, err := r.GetIndex()
	if err != nil {
		return fmt.Errorf("read index: %w", err)
	}
	if len(index) == 0 {
		return fmt.Errorf("nothing to commit (working tree clean / index is empty)")
	}

	treeHash, err := object.BuildTree(index, r.LocalStore())
	if err != nil {
		return fmt.Errorf("build tree: %w", err)
	}

	parentCommit, _ := r.ResolveHead()

	if author == "" {
		author = os.Getenv("ZIT_AUTHOR")
	}
	if author == "" {
		author = "Zit User <user@zit.local>"
	}

	c := &object.Commit{
		Tree:      treeHash,
		Parent:    parentCommit,
		Author:    author,
		Timestamp: time.Now().Unix(),
		Message:   message,
	}

	cObj := object.NewObject(object.TypeCommit, c.Serialize())
	if _, err := r.LocalStore().Put(cObj.RawData); err != nil {
		return fmt.Errorf("save commit: %w", err)
	}

	headRef, isBranch, err := r.GetHead()
	if err != nil {
		return fmt.Errorf("get head: %w", err)
	}

	branchName := "detached"
	if isBranch {
		branchName = strings.TrimPrefix(headRef, "refs/heads/")
		if err := r.SetRef(headRef, cObj.Hash); err != nil {
			return fmt.Errorf("update ref %s: %w", headRef, err)
		}
	} else {
		if err := r.SetHeadCommit(cObj.Hash); err != nil {
			return fmt.Errorf("update HEAD: %w", err)
		}
	}

	shortHash := cObj.Hash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}
	fmt.Printf("[%s %s] %s\n", branchName, shortHash, strings.Split(message, "\n")[0])
	return nil
}
