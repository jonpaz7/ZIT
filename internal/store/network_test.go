package store_test

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"zit/internal/crypto"
	"zit/internal/object"
	"zit/internal/store"
)

type mockTracker struct {
	providers map[string][]string
}

func (m *mockTracker) Providers(hash string) ([]string, error) {
	return m.providers[hash], nil
}

type mockPeerCli struct {
	data map[string]map[string][]byte // peer -> hash -> data
}

func (m *mockPeerCli) Fetch(peerAddr, hash string) ([]byte, error) {
	if peerData, ok := m.data[peerAddr]; ok {
		if raw, ok := peerData[hash]; ok {
			return raw, nil
		}
	}
	return nil, errors.New("not found on peer")
}

func TestNetworkStoreGetAndCache(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "zit_net_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	ls := store.NewLocalStore(tempDir)
	raw := object.FormatRaw(object.TypeBlob, []byte("remote blob payload"))
	hash := crypto.Hash(raw)

	tr := &mockTracker{
		providers: map[string][]string{
			hash: {"peer1:8001", "peer2:8002"},
		},
	}
	peerCli := &mockPeerCli{
		data: map[string]map[string][]byte{
			"peer1:8001": {hash: raw},
			"peer2:8002": {hash: raw},
		},
	}

	ns := store.NewNetworkStore(ls, tr, peerCli)

	// Local should not have it yet
	if ls.Has(hash) {
		t.Fatal("expected local store not to have hash yet")
	}

	// Fetch via NetworkStore
	got, err := ns.Get(hash)
	if err != nil {
		t.Fatalf("ns.Get failed: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("got data mismatch")
	}

	// Now it MUST be cached in local store
	if !ls.Has(hash) {
		t.Fatal("expected local store to have cached the object")
	}

	// Second fetch should come directly from local store
	cached, err := ns.Get(hash)
	if err != nil {
		t.Fatalf("ns.Get cached failed: %v", err)
	}
	if !bytes.Equal(cached, raw) {
		t.Fatalf("cached data mismatch")
	}
}

func TestNetworkStoreCorruptPeerFallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "zit_net_corrupt_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	ls := store.NewLocalStore(tempDir)
	validRaw := object.FormatRaw(object.TypeBlob, []byte("valid payload"))
	hash := crypto.Hash(validRaw)

	corruptRaw := object.FormatRaw(object.TypeBlob, []byte("tampered bad payload"))

	tr := &mockTracker{
		providers: map[string][]string{
			hash: {"bad_peer:8001", "good_peer:8002"},
		},
	}
	peerCli := &mockPeerCli{
		data: map[string]map[string][]byte{
			"bad_peer:8001":  {hash: corruptRaw},
			"good_peer:8002": {hash: validRaw},
		},
	}

	ns := store.NewNetworkStore(ls, tr, peerCli)

	got, err := ns.Get(hash)
	if err != nil {
		t.Fatalf("expected fallback to good peer, got error: %v", err)
	}
	if !bytes.Equal(got, validRaw) {
		t.Fatalf("expected valid payload from good peer")
	}
}
