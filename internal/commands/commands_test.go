package commands_test

import (
	"os"
	"testing"
	"zit/internal/commands"
	"zit/internal/repo"
)

func TestLocalGitWorkflow(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	tempDir, err := os.MkdirTemp("", "zit_local_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	// 1. Init
	if err := commands.Init(nil, "", "127.0.0.1:8001"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// 2. Create files
	_ = os.WriteFile("file1.txt", []byte("version 1 on main"), 0644)
	_ = os.MkdirAll("sub", 0755)
	_ = os.WriteFile("sub/file2.txt", []byte("sub content v1"), 0644)

	// 3. Add and Commit
	if err := commands.Add([]string{"file1.txt", "sub"}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if err := commands.Commit("initial commit on main", "Tester <test@zit.local>"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	r, err := repo.Find(".")
	if err != nil {
		t.Fatal(err)
	}
	headCommit1, err := r.ResolveHead()
	if err != nil || headCommit1 == "" {
		t.Fatalf("invalid head commit: %v", err)
	}

	// 4. Create branch feature
	if err := commands.Branch("feature"); err != nil {
		t.Fatalf("Branch failed: %v", err)
	}

	// 5. Checkout feature branch
	if err := commands.Checkout([]string{"feature"}); err != nil {
		t.Fatalf("Checkout feature failed: %v", err)
	}

	// 6. Modify file on feature
	_ = os.WriteFile("file1.txt", []byte("version 2 on feature branch"), 0644)
	_ = os.WriteFile("feature_only.txt", []byte("feature file"), 0644)

	if err := commands.Add([]string{"file1.txt", "feature_only.txt"}); err != nil {
		t.Fatalf("Add on feature failed: %v", err)
	}

	if err := commands.Commit("feature commit", "Feature Dev <dev@zit.local>"); err != nil {
		t.Fatalf("Commit on feature failed: %v", err)
	}

	// 7. Switch back to main
	if err := commands.Checkout([]string{"main"}); err != nil {
		t.Fatalf("Checkout main failed: %v", err)
	}

	// Verify file1.txt restored to version 1
	data, err := os.ReadFile("file1.txt")
	if err != nil || string(data) != "version 1 on main" {
		t.Fatalf("file1.txt not restored to main content: got '%s'", string(data))
	}

	// Verify feature_only.txt was removed on main
	if _, err := os.Stat("feature_only.txt"); !os.IsNotExist(err) {
		t.Fatalf("feature_only.txt should not exist on main branch")
	}

	// 8. Switch back to feature
	if err := commands.Checkout([]string{"feature"}); err != nil {
		t.Fatalf("Checkout feature failed: %v", err)
	}

	data, err = os.ReadFile("file1.txt")
	if err != nil || string(data) != "version 2 on feature branch" {
		t.Fatalf("file1.txt not updated to feature content: got '%s'", string(data))
	}
	if _, err := os.Stat("feature_only.txt"); err != nil {
		t.Fatalf("feature_only.txt should exist on feature branch")
	}
}
