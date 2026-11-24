package commands

import (
	"fmt"
	"strings"
	"zit/internal/crypto"
	"zit/internal/repo"
	"zit/internal/tracker"
)

func Publish() error {
	r, err := repo.Find(".")
	if err != nil {
		return err
	}

	if r.Config.Tracker == "" {
		return fmt.Errorf("no tracker configured in %s", repo.ConfigFile)
	}
	if r.Config.RepoID == "" {
		return fmt.Errorf("no repo_id configured in %s", repo.ConfigFile)
	}

	pub, priv, err := r.GetKeys()
	if err != nil {
		return fmt.Errorf("read keys: %w", err)
	}
	if priv == nil {
		// Non-owner replica: announce local objects so this peer can seed
		hashes, _ := r.LocalStore().List()
		if len(hashes) > 0 {
			peerAddr := r.Config.Peer
			if peerAddr == "" {
				peerAddr = "127.0.0.1:8001"
			}
			trackerCli := tracker.NewClient(r.Config.Tracker)
			if err := trackerCli.Announce(peerAddr, hashes); err != nil {
				return fmt.Errorf("announce objects: %w", err)
			}
			fmt.Printf("Announced %d objects to tracker (%s) as peer %s\n", len(hashes), r.Config.Tracker, peerAddr)
		}
		fmt.Printf("Notice: replica does not have private key. Local objects announced as seeder; ref update skipped.\n")
		return nil
	}

	headRef, isBranch, err := r.GetHead()
	if err != nil {
		return fmt.Errorf("get HEAD: %w", err)
	}
	if !isBranch {
		return fmt.Errorf("cannot publish detached HEAD; checkout a branch first")
	}

	commitHash, err := r.ResolveHead()
	if err != nil || commitHash == "" {
		return fmt.Errorf("branch '%s' has no commits to publish", headRef)
	}

	trackerCli := tracker.NewClient(r.Config.Tracker)

	// 1. Announce all local objects to tracker
	peerAddr := r.Config.Peer
	if peerAddr == "" {
		peerAddr = "127.0.0.1:8001"
	}
	hashes, err := r.LocalStore().List()
	if err != nil {
		return fmt.Errorf("list objects: %w", err)
	}
	if len(hashes) > 0 {
		if err := trackerCli.Announce(peerAddr, hashes); err != nil {
			return fmt.Errorf("announce objects to tracker: %w", err)
		}
		fmt.Printf("Announced %d objects to tracker (%s) as peer %s\n", len(hashes), r.Config.Tracker, peerAddr)
	}

	// 2. Sign ref and update tracker
	msg := []byte(fmt.Sprintf("%s:%s:%s", r.Config.RepoID, headRef, commitHash))
	sig := crypto.Sign(priv, msg)
	pubHex := crypto.EncodePublicKey(pub)

	if err := trackerCli.PutRef(r.Config.RepoID, headRef, commitHash, sig, pubHex); err != nil {
		return fmt.Errorf("failed to publish ref to tracker: %w", err)
	}

	branchName := strings.TrimPrefix(headRef, "refs/heads/")
	short := commitHash
	if len(short) > 7 {
		short = short[:7]
	}
	fmt.Printf("Published branch '%s' -> %s to %s\n", branchName, short, r.Config.Tracker)
	return nil
}
