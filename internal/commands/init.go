package commands

import (
	"fmt"
	"path/filepath"
	"zit/internal/repo"
)

func Init(args []string, trackerURL, peerAddr string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	r, err := repo.Init(abs, trackerURL, peerAddr)
	if err != nil {
		return err
	}
	fmt.Printf("Initialized empty ZIT repository in %s\n", r.ZitPath)
	fmt.Printf("Repo ID: %s\n", r.Config.RepoID)
	return nil
}
