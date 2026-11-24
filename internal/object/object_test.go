package object_test

import (
	"bytes"
	"errors"
	"testing"
	"time"
	"zit/internal/crypto"
	"zit/internal/object"
)

func TestBlobObject(t *testing.T) {
	content := []byte("hello blob content")
	obj := object.NewObject(object.TypeBlob, content)

	if obj.Type != object.TypeBlob {
		t.Fatalf("expected blob, got %s", obj.Type)
	}

	compressed, err := object.Compress(obj.RawData)
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}

	decompressed, err := object.Decompress(compressed)
	if err != nil {
		t.Fatalf("decompress failed: %v", err)
	}

	parsed, err := object.ParseRaw(decompressed)
	if err != nil {
		t.Fatalf("ParseRaw failed: %v", err)
	}

	if parsed.Hash != obj.Hash {
		t.Fatalf("hash mismatch: expected %s, got %s", obj.Hash, parsed.Hash)
	}
	if !bytes.Equal(parsed.Data, content) {
		t.Fatalf("content mismatch")
	}
}

func TestTreeObject(t *testing.T) {
	entries := []object.TreeEntry{
		{Mode: object.ModeBlob, Name: "file2.txt", Hash: "b2"},
		{Mode: object.ModeBlob, Name: "file1.txt", Hash: "a1"},
		{Mode: object.ModeTree, Name: "subdir", Hash: "c3"},
	}
	treeData := object.SerializeTree(entries)
	parsedTree, err := object.ParseTree(treeData)
	if err != nil {
		t.Fatalf("ParseTree failed: %v", err)
	}

	if len(parsedTree.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(parsedTree.Entries))
	}
	// Verify sorting by name: file1.txt, file2.txt, subdir
	if parsedTree.Entries[0].Name != "file1.txt" ||
		parsedTree.Entries[1].Name != "file2.txt" ||
		parsedTree.Entries[2].Name != "subdir" {
		t.Fatalf("tree entries not sorted correctly: %+v", parsedTree.Entries)
	}
}

func TestCommitObject(t *testing.T) {
	commit := &object.Commit{
		Tree:      "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		Parent:    "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Author:    "Test Author <test@zit.local>",
		Timestamp: time.Now().Unix(),
		Message:   "Initial commit\n\nDetailed description here.",
	}

	commitBytes := commit.Serialize()
	parsed, err := object.ParseCommit(commitBytes)
	if err != nil {
		t.Fatalf("ParseCommit failed: %v", err)
	}

	if parsed.Tree != commit.Tree || parsed.Parent != commit.Parent || parsed.Author != commit.Author || parsed.Message != commit.Message {
		t.Fatalf("commit fields mismatch: %+v vs %+v", parsed, commit)
	}
}

type memStore struct {
	m map[string][]byte
}

var errNotFound = errors.New("not found")

func (s *memStore) Has(h string) bool {
	_, ok := s.m[h]
	return ok
}
func (s *memStore) Get(h string) ([]byte, error) {
	v, ok := s.m[h]
	if !ok {
		return nil, errNotFound
	}
	return v, nil
}
func (s *memStore) Put(obj []byte) (string, error) {
	h := crypto.Hash(obj)
	s.m[h] = obj
	return h, nil
}

func TestBuildAndFlattenTree(t *testing.T) {
	ms := &memStore{m: make(map[string][]byte)}

	index := map[string]string{
		"README.md":           "blob_readme",
		"src/main.go":         "blob_main",
		"src/util/helper.go":  "blob_helper",
		"docs/guide/start.md": "blob_start",
	}

	rootHash, err := object.BuildTree(index, ms)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	flattened, err := object.FlattenTree(rootHash, ms)
	if err != nil {
		t.Fatalf("FlattenTree failed: %v", err)
	}

	if len(flattened) != len(index) {
		t.Fatalf("expected %d entries, got %d", len(index), len(flattened))
	}

	for k, v := range index {
		if flattened[k] != v {
			t.Fatalf("mismatch for %s: expected %s, got %s", k, v, flattened[k])
		}
	}
}
