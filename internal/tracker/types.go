package tracker

import "time"

// AnnounceRequest payload sent by peers to register objects.
type AnnounceRequest struct {
	Hashes []string `json:"hashes"`
	Peer   string   `json:"peer"` // host:port
}

// AnnounceResponse returned after successful announce.
type AnnounceResponse struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// RefData represents a signed git ref.
type RefData struct {
	Commit    string    `json:"commit"`
	Signature string    `json:"signature"`
	PublicKey string    `json:"public_key,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RepoRefs maps ref names (e.g. "refs/heads/main") to RefData.
type RepoRefs map[string]*RefData
