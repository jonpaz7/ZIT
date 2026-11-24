package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// GenerateKeyPair generates a new Ed25519 keypair.
func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate ed25519 key: %w", err)
	}
	return pub, priv, nil
}

// RepoID computes the repo ID as the SHA-256 hex digest of the public key.
func RepoID(pub ed25519.PublicKey) string {
	return Hash(pub)
}

// Hash returns the hex-encoded SHA-256 checksum of data.
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Sign signs a message using an Ed25519 private key and returns the hex-encoded signature.
func Sign(priv ed25519.PrivateKey, message []byte) string {
	sig := ed25519.Sign(priv, message)
	return hex.EncodeToString(sig)
}

// Verify verifies an Ed25519 signature against a public key.
func Verify(pub ed25519.PublicKey, message []byte, sigHex string) bool {
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	return ed25519.Verify(pub, message, sig)
}

// EncodePublicKey returns the hex string of a public key.
func EncodePublicKey(pub ed25519.PublicKey) string {
	return hex.EncodeToString(pub)
}

// DecodePublicKey parses a hex string into an Ed25519 public key.
func DecodePublicKey(s string) (ed25519.PublicKey, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, errors.New("invalid public key length")
	}
	return ed25519.PublicKey(b), nil
}

// EncodePrivateKey returns the hex string of a private key.
func EncodePrivateKey(priv ed25519.PrivateKey) string {
	return hex.EncodeToString(priv)
}

// DecodePrivateKey parses a hex string into an Ed25519 private key.
func DecodePrivateKey(s string) (ed25519.PrivateKey, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid private key length")
	}
	return ed25519.PrivateKey(b), nil
}
