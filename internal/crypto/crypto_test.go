package crypto_test

import (
	"testing"
	"zit/internal/crypto"
)

func TestCryptoKeyPairAndSign(t *testing.T) {
	pub, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	repoID := crypto.RepoID(pub)
	if len(repoID) != 64 {
		t.Fatalf("expected 64 char repoID hex, got %s", repoID)
	}

	msg := []byte("hello world")
	sig := crypto.Sign(priv, msg)
	if !crypto.Verify(pub, msg, sig) {
		t.Fatalf("verification failed for valid signature")
	}

	tamperedMsg := []byte("hello world modified")
	if crypto.Verify(pub, tamperedMsg, sig) {
		t.Fatalf("verification should have failed for tampered message")
	}

	// Test encode / decode
	pubHex := crypto.EncodePublicKey(pub)
	decodedPub, err := crypto.DecodePublicKey(pubHex)
	if err != nil {
		t.Fatalf("DecodePublicKey failed: %v", err)
	}
	if crypto.RepoID(decodedPub) != repoID {
		t.Fatalf("repoID mismatch after decode")
	}
}
