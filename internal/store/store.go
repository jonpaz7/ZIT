package store

import "errors"

var (
	ErrNotFound  = errors.New("object not found")
	ErrCorrupted = errors.New("object hash mismatch or corrupt")
)

// ObjectStore is the central interface for reading and writing ZIT objects.
type ObjectStore interface {
	Has(hash string) bool
	Get(hash string) ([]byte, error)
	Put(obj []byte) (hash string, err error)
}
