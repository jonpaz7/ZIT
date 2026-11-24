package tracker_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"zit/internal/crypto"
	"zit/internal/tracker"
)

type inMemoryTransport struct {
	handler http.Handler
}

func (t *inMemoryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	t.handler.ServeHTTP(rec, req)
	return rec.Result(), nil
}

func TestTrackerAnnounceAndProviders(t *testing.T) {
	srv := tracker.NewServer("")
	httpClient := &http.Client{
		Transport: &inMemoryTransport{handler: srv.Handler()},
	}
	client := tracker.NewClientWithHTTPClient("http://mock-tracker", httpClient)

	peer1 := "127.0.0.1:8001"
	hashes := []string{"hash1", "hash2"}

	err := client.Announce(peer1, hashes)
	if err != nil {
		t.Fatalf("Announce failed: %v", err)
	}

	providers1, err := client.Providers("hash1")
	if err != nil {
		t.Fatalf("Providers failed: %v", err)
	}
	if len(providers1) != 1 || providers1[0] != peer1 {
		t.Fatalf("expected [%s], got %v", peer1, providers1)
	}

	providersUnknown, err := client.Providers("nonexistent")
	if err != nil {
		t.Fatalf("Providers failed: %v", err)
	}
	if len(providersUnknown) != 0 {
		t.Fatalf("expected empty providers, got %v", providersUnknown)
	}
}

func TestTrackerRefSignatures(t *testing.T) {
	srv := tracker.NewServer("")
	httpClient := &http.Client{
		Transport: &inMemoryTransport{handler: srv.Handler()},
	}
	client := tracker.NewClientWithHTTPClient("http://mock-tracker", httpClient)

	pub, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	repoID := crypto.RepoID(pub)
	pubHex := crypto.EncodePublicKey(pub)

	ref := "refs/heads/main"
	commit := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	// Sign correctly
	msg := []byte(fmt.Sprintf("%s:%s:%s", repoID, ref, commit))
	sig := crypto.Sign(priv, msg)

	err = client.PutRef(repoID, ref, commit, sig, pubHex)
	if err != nil {
		t.Fatalf("PutRef failed: %v", err)
	}

	// Verify GetRefs
	refs, err := client.GetRefs(repoID)
	if err != nil {
		t.Fatalf("GetRefs failed: %v", err)
	}
	refData, ok := refs[ref]
	if !ok {
		t.Fatalf("ref %s not found in returned refs: %+v", ref, refs)
	}
	if refData.Commit != commit {
		t.Fatalf("commit mismatch: %s != %s", refData.Commit, commit)
	}

	// Test invalid signature rejection
	badSig := crypto.Sign(priv, []byte("wrong message"))
	err = client.PutRef(repoID, ref, "newcommit", badSig, pubHex)
	if err == nil {
		t.Fatalf("expected error for bad signature, got nil")
	}

	// Test mismatched repo ID rejection
	_, otherPriv, _ := crypto.GenerateKeyPair()
	otherSig := crypto.Sign(otherPriv, []byte(fmt.Sprintf("%s:%s:%s", "differentRepo", ref, commit)))
	err = client.PutRef("differentRepo", ref, commit, otherSig, pubHex)
	if err == nil {
		t.Fatalf("expected error for mismatched repoId, got nil")
	}
}
