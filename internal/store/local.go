package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"zit/internal/crypto"
	"zit/internal/object"
)

// LocalStore manages objects saved on the local disk under .zit/objects.
type LocalStore struct {
	dir string
}

func NewLocalStore(dir string) *LocalStore {
	return &LocalStore{dir: dir}
}

func (s *LocalStore) objectPath(hash string) string {
	if len(hash) < 3 {
		return filepath.Join(s.dir, hash)
	}
	return filepath.Join(s.dir, hash[:2], hash[2:])
}

func (s *LocalStore) Has(hash string) bool {
	p := s.objectPath(hash)
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func (s *LocalStore) Get(hash string) ([]byte, error) {
	p := s.objectPath(hash)
	compressed, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	uncompressed, err := object.Decompress(compressed)
	if err != nil {
		return nil, fmt.Errorf("decompress object %s: %w", hash, err)
	}
	if crypto.Hash(uncompressed) != hash {
		return nil, fmt.Errorf("%w for %s", ErrCorrupted, hash)
	}
	return uncompressed, nil
}

func (s *LocalStore) Put(obj []byte) (string, error) {
	h := crypto.Hash(obj)
	p := s.objectPath(h)
	if s.Has(h) {
		return h, nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return "", err
	}
	compressed, err := object.Compress(obj)
	if err != nil {
		return "", fmt.Errorf("compress object: %w", err)
	}
	tmpFile := p + ".tmp"
	if err := os.WriteFile(tmpFile, compressed, 0644); err != nil {
		return "", err
	}
	if err := os.Rename(tmpFile, p); err != nil {
		// on Windows, remove existing if collision before rename
		os.Remove(p)
		if err := os.Rename(tmpFile, p); err != nil {
			os.Remove(tmpFile)
			return "", err
		}
	}
	return h, nil
}

// GetCompressed reads raw compressed bytes directly from disk.
func (s *LocalStore) GetCompressed(hash string) ([]byte, error) {
	p := s.objectPath(hash)
	compressed, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return compressed, nil
}

// PutCompressed verifies decompressed content matches hash, then saves the compressed payload.
func (s *LocalStore) PutCompressed(hash string, compressed []byte) error {
	uncompressed, err := object.Decompress(compressed)
	if err != nil {
		return fmt.Errorf("invalid compressed data for %s: %w", hash, err)
	}
	if crypto.Hash(uncompressed) != hash {
		return fmt.Errorf("%w for %s", ErrCorrupted, hash)
	}
	p := s.objectPath(hash)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	tmpFile := p + ".tmp"
	if err := os.WriteFile(tmpFile, compressed, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpFile, p); err != nil {
		os.Remove(p)
		if err := os.Rename(tmpFile, p); err != nil {
			os.Remove(tmpFile)
			return err
		}
	}
	return nil
}

// List returns all valid object hashes stored locally.
func (s *LocalStore) List() ([]string, error) {
	var hashes []string
	prefixEntries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	for _, pe := range prefixEntries {
		if !pe.IsDir() || len(pe.Name()) != 2 {
			continue
		}
		subDir := filepath.Join(s.dir, pe.Name())
		files, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || strings.HasSuffix(f.Name(), ".tmp") {
				continue
			}
			fullHash := pe.Name() + f.Name()
			if len(fullHash) == 64 {
				hashes = append(hashes, fullHash)
			}
		}
	}
	return hashes, nil
}
