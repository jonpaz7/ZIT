package peer_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"zit/internal/object"
	"zit/internal/peer"
	"zit/internal/store"
)

type inMemoryTransport struct {
	handler http.Handler
}

func (t *inMemoryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	t.handler.ServeHTTP(rec, req)
	return rec.Result(), nil
}

func TestPeerFetchAndVerify(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "zit_peer_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	localStore := store.NewLocalStore(tempDir)
	raw := object.FormatRaw(object.TypeBlob, []byte("peer data test"))
	hash, err := localStore.Put(raw)
	if err != nil {
		t.Fatal(err)
	}

	peerSrv := peer.NewServer("peer-addr", localStore, nil)
	httpClient := &http.Client{
		Transport: &inMemoryTransport{handler: peerSrv.Handler()},
	}
	client := peer.NewClientWithHTTPClient(httpClient)

	// Fetch valid object
	data, err := client.Fetch("http://peer-addr", hash)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if !bytes.Equal(data, raw) {
		t.Fatalf("data mismatch")
	}

	// Fetch missing object
	_, err = client.Fetch("http://peer-addr", "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatalf("expected error for missing object, got nil")
	}
}
