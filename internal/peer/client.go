package peer

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"zit/internal/crypto"
	"zit/internal/object"
	"zit/internal/store"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return NewClientWithHTTPClient(&http.Client{
		Timeout: 15 * time.Second,
	})
}

func NewClientWithHTTPClient(httpClient *http.Client) *Client {
	return &Client{
		httpClient: httpClient,
	}
}

// Fetch retrieves and verifies an object from a peer.
// Returns the verified uncompressed object data bytes (<type> <size>\0<content>).
func (c *Client) Fetch(peerAddr, hash string) ([]byte, error) {
	if !strings.HasPrefix(peerAddr, "http://") && !strings.HasPrefix(peerAddr, "https://") {
		peerAddr = "http://" + peerAddr
	}
	url := fmt.Sprintf("%s/objects/%s", strings.TrimRight(peerAddr, "/"), hash)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch from %s: %w", peerAddr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, store.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("peer returned status %d: %s", resp.StatusCode, string(body))
	}

	compressed, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body from %s: %w", peerAddr, err)
	}

	uncompressed, err := object.Decompress(compressed)
	if err != nil {
		return nil, fmt.Errorf("decompress error from peer %s: %w", peerAddr, err)
	}

	calculatedHash := crypto.Hash(uncompressed)
	if calculatedHash != hash {
		return nil, fmt.Errorf("%w: peer %s returned hash %s, expected %s", store.ErrCorrupted, peerAddr, calculatedHash, hash)
	}

	return uncompressed, nil
}
