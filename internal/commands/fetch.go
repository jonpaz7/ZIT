package commands

import (
	"fmt"
	"strings"
	"zit/internal/crypto"
	"zit/internal/object"
	"zit/internal/repo"
	"zit/internal/tracker"
)

func Fetch() error {
	r, err := repo.Find(".")
	if err != nil {
		return err
	}

	if r.Config.Tracker == "" || r.Config.RepoID == "" {
		return fmt.Errorf("tracker or repo_id missing in configuration")
	}

	trackerCli := tracker.NewClient(r.Config.Tracker)
	refs, err := trackerCli.GetRefs(r.Config.RepoID)
	if err != nil {
		return fmt.Errorf("fetch refs from tracker: %w", err)
	}

	pub, _, _ := r.GetKeys()

	updatedCount := 0
	for refName, refData := range refs {
		// Verify signature if public key is known or provided
		verifyPub := pub
		if verifyPub == nil && refData.PublicKey != "" {
			decodedPub, err := crypto.DecodePublicKey(refData.PublicKey)
			if err == nil && crypto.RepoID(decodedPub) == r.Config.RepoID {
				verifyPub = decodedPub
			}
		}

		if verifyPub != nil {
			msg := []byte(fmt.Sprintf("%s:%s:%s", r.Config.RepoID, refName, refData.Commit))
			if !crypto.Verify(verifyPub, msg, refData.Signature) && !crypto.Verify(verifyPub, []byte(refData.Commit), refData.Signature) {
				return fmt.Errorf("signature verification failed for ref %s", refName)
			}
		}

		localCommit, _ := r.GetRef(refName)
		if localCommit == refData.Commit {
			continue // up to date
		}

		// Download commit and trees
		rawCommit, err := r.Store().Get(refData.Commit)
		if err != nil {
			return fmt.Errorf("fetch commit %s: %w", refData.Commit, err)
		}
		cObj, err := object.ParseRaw(rawCommit)
		if err != nil {
			return fmt.Errorf("parse commit %s: %w", refData.Commit, err)
		}
		commit, err := object.ParseCommit(cObj.Data)
		if err != nil {
			return fmt.Errorf("parse commit fields %s: %w", refData.Commit, err)
		}

		// Download all tree objects in this commit
		if _, err := object.FlattenTree(commit.Tree, r.Store()); err != nil {
			return fmt.Errorf("fetch trees for %s: %w", refData.Commit, err)
		}

		// Update local ref
		if err := r.SetRef(refName, refData.Commit); err != nil {
			return fmt.Errorf("update ref %s: %w", refName, err)
		}

		branchName := strings.TrimPrefix(refName, "refs/heads/")
		short := refData.Commit
		if len(short) > 7 {
			short = short[:7]
		}
		fmt.Printf("Updated %s -> %s\n", branchName, short)
		updatedCount++
	}

	if updatedCount == 0 {
		fmt.Println("Already up to date.")
	} else {
		fmt.Printf("Fetched %d updated refs (commits & trees downloaded, blobs remain lazy).\n", updatedCount)
	}

	return nil
}
