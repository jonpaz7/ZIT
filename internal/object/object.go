package object

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"zit/internal/crypto"
)

type Type string

const (
	TypeBlob   Type = "blob"
	TypeTree   Type = "tree"
	TypeCommit Type = "commit"

	ModeBlob = "100644"
	ModeTree = "040000"
)

var (
	ErrInvalidObject = errors.New("invalid object format")
	ErrCorruptObject = errors.New("corrupt object or hash mismatch")
)

// Object represents a decoded Git/ZIT object.
type Object struct {
	Type    Type
	Data    []byte
	Hash    string
	RawData []byte // the uncompressed <type> <size>\0<content>
}

// FormatRaw prepares `<type> <size>\0<content>`.
func FormatRaw(objType Type, content []byte) []byte {
	header := fmt.Sprintf("%s %d\x00", objType, len(content))
	raw := make([]byte, 0, len(header)+len(content))
	raw = append(raw, []byte(header)...)
	raw = append(raw, content...)
	return raw
}

// NewObject creates an Object from type and content bytes.
func NewObject(objType Type, content []byte) *Object {
	raw := FormatRaw(objType, content)
	h := crypto.Hash(raw)
	return &Object{
		Type:    objType,
		Data:    content,
		Hash:    h,
		RawData: raw,
	}
}

// ParseRaw parses uncompressed `<type> <size>\0<content>` bytes.
func ParseRaw(raw []byte) (*Object, error) {
	nullIdx := bytes.IndexByte(raw, 0)
	if nullIdx < 0 {
		return nil, ErrInvalidObject
	}
	header := string(raw[:nullIdx])
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidObject
	}
	objType := Type(parts[0])
	size, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, ErrInvalidObject
	}
	data := raw[nullIdx+1:]
	if len(data) != size {
		return nil, fmt.Errorf("%w: declared size %d != actual %d", ErrInvalidObject, size, len(data))
	}

	h := crypto.Hash(raw)
	return &Object{
		Type:    objType,
		Data:    data,
		Hash:    h,
		RawData: raw,
	}, nil
}

// Compress compresses bytes using zlib.
func Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decompress decompresses zlib compressed bytes.
func Decompress(compressed []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// TreeEntry represents an entry inside a tree object.
type TreeEntry struct {
	Mode string `json:"mode"`
	Name string `json:"name"`
	Hash string `json:"hash"`
}

// Tree represents a directory tree.
type Tree struct {
	Entries []TreeEntry
}

// SerializeTree serializes entries to tree content, sorted by entry Name.
func SerializeTree(entries []TreeEntry) []byte {
	sorted := make([]TreeEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	var buf bytes.Buffer
	for _, e := range sorted {
		fmt.Fprintf(&buf, "%s %s %s\n", e.Mode, e.Name, e.Hash)
	}
	return buf.Bytes()
}

// ParseTree parses the content of a tree object.
func ParseTree(content []byte) (*Tree, error) {
	tree := &Tree{}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("malformed tree entry: %s", line)
		}
		tree.Entries = append(tree.Entries, TreeEntry{
			Mode: parts[0],
			Name: parts[1],
			Hash: parts[2],
		})
	}
	return tree, nil
}

// Commit represents a commit object.
type Commit struct {
	Tree      string
	Parent    string
	Author    string
	Timestamp int64
	Message   string
}

// Serialize serializes a commit to bytes.
func (c *Commit) Serialize() []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "tree %s\n", c.Tree)
	if c.Parent != "" {
		fmt.Fprintf(&buf, "parent %s\n", c.Parent)
	}
	fmt.Fprintf(&buf, "author %s\n", c.Author)
	fmt.Fprintf(&buf, "timestamp %d\n", c.Timestamp)
	fmt.Fprintf(&buf, "\n%s\n", strings.TrimSpace(c.Message))
	return buf.Bytes()
}

// ParseCommit parses the content of a commit object.
func ParseCommit(content []byte) (*Commit, error) {
	commit := &Commit{}
	str := string(content)
	parts := strings.SplitN(str, "\n\n", 2)
	if len(parts) < 2 {
		return nil, errors.New("malformed commit format: missing message separator")
	}
	headerLines := strings.Split(parts[0], "\n")
	commit.Message = strings.TrimSpace(parts[1])

	for _, line := range headerLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		kv := strings.SplitN(line, " ", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "tree":
			commit.Tree = kv[1]
		case "parent":
			commit.Parent = kv[1]
		case "author":
			commit.Author = kv[1]
		case "timestamp":
			ts, err := strconv.ParseInt(kv[1], 10, 64)
			if err == nil {
				commit.Timestamp = ts
			}
		}
	}

	if commit.Tree == "" {
		return nil, errors.New("commit missing tree hash")
	}
	return commit, nil
}
