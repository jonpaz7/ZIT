package store_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"zit/internal/object"
	"zit/internal/store"
)

func TestLocalStorePutAndGet(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "zit_store_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	ls := store.NewLocalStore(tempDir)

	raw := object.FormatRaw(object.TypeBlob, []byte("test content 123"))
	h, err := ls.Put(raw)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if !ls.Has(h) {
		t.Fatalf("expected Has(%s) to be true", h)
	}

	got, err := ls.Get(h)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("content mismatch")
	}

	hashes, err := ls.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(hashes) != 1 || hashes[0] != h {
		t.Fatalf("expected [%s], got %v", h, hashes)
	}

	// Corrupt test
	p := filepath.Join(tempDir, h[:2], h[2:])
	badData, _ := object.Compress([]byte("corrupt data"))
	_ = os.WriteFile(p, badData, 0644)

	_, err = ls.Get(h)
	if err == nil {
		t.Fatalf("expected corruption error, got nil")
	}
}
