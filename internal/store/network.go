package store

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"zit/internal/crypto"
)

type ProviderLookup interface {
	Providers(hash string) ([]string, error)
}

type PeerFetcher interface {
	Fetch(peerAddr, hash string) ([]byte, error)
}

// NetworkStore coordinates local cache and remote peer retrieval via tracker.
type NetworkStore struct {
	local   *LocalStore
	tracker ProviderLookup
	peerCli PeerFetcher
}

func NewNetworkStore(local *LocalStore, trackerCli ProviderLookup, peerCli PeerFetcher) *NetworkStore {
	return &NetworkStore{
		local:   local,
		tracker: trackerCli,
		peerCli: peerCli,
	}
}

func (n *NetworkStore) Local() *LocalStore {
	return n.local
}

func (n *NetworkStore) Has(hash string) bool {
	return n.local.Has(hash)
}

func (n *NetworkStore) Put(obj []byte) (string, error) {
	return n.local.Put(obj)
}

func (n *NetworkStore) Get(hash string) ([]byte, error) {
	if n.local.Has(hash) {
		return n.local.Get(hash)
	}
	if n.tracker == nil || n.peerCli == nil {
		return nil, fmt.Errorf("%w: object %s (no tracker/peer client configured)", ErrNotFound, hash)
	}

	peers, err := n.tracker.Providers(hash)
	if err != nil {
		return nil, fmt.Errorf("lookup providers for %s: %w", hash, err)
	}
	if len(peers) == 0 {
		return nil, fmt.Errorf("%w: object %s (no providers)", ErrNotFound, hash)
	}

	shuffled := shufflePeers(peers)
	for _, p := range shuffled {
		data, err := n.peerCli.Fetch(p, hash)
		if err == nil && crypto.Hash(data) == hash {
			// Cache locally to serve as seeder
			if _, putErr := n.local.Put(data); putErr != nil {
				fmt.Printf("warning: failed to cache downloaded object %s: %v\n", hash, putErr)
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("%w: object %s from %d peers", ErrNotFound, hash, len(peers))
}

func shufflePeers(peers []string) []string {
	result := make([]string, len(peers))
	copy(result, peers)
	for i := len(result) - 1; i > 0; i-- {
		nBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			continue
		}
		j := int(nBig.Int64())
		result[i], result[j] = result[j], result[i]
	}
	return result
}
