package object

import (
	"fmt"
	"path"
	"strings"
)

// Store is the minimal interface needed to build and read trees.
type Store interface {
	Get(hash string) ([]byte, error)
	Put(obj []byte) (string, error)
}

type treeNode struct {
	blobs   map[string]string
	subdirs map[string]*treeNode
}

func newTreeNode() *treeNode {
	return &treeNode{
		blobs:   make(map[string]string),
		subdirs: make(map[string]*treeNode),
	}
}

// BuildTree builds a hierarchy of tree objects from an index map (path -> blobHash)
// and saves all tree objects into the provided Store.
// Returns the root tree hash.
func BuildTree(index map[string]string, s Store) (string, error) {
	root := newTreeNode()
	for p, blobHash := range index {
		clean := path.Clean(strings.ReplaceAll(p, "\\", "/"))
		if clean == "." || clean == "" {
			continue
		}
		parts := strings.Split(clean, "/")
		curr := root
		for i := 0; i < len(parts)-1; i++ {
			dirName := parts[i]
			if _, ok := curr.subdirs[dirName]; !ok {
				curr.subdirs[dirName] = newTreeNode()
			}
			curr = curr.subdirs[dirName]
		}
		curr.blobs[parts[len(parts)-1]] = blobHash
	}

	return saveNode(root, s)
}

func saveNode(n *treeNode, s Store) (string, error) {
	var entries []TreeEntry

	for dirName, sub := range n.subdirs {
		subHash, err := saveNode(sub, s)
		if err != nil {
			return "", err
		}
		entries = append(entries, TreeEntry{
			Mode: ModeTree,
			Name: dirName,
			Hash: subHash,
		})
	}

	for fileName, blobHash := range n.blobs {
		entries = append(entries, TreeEntry{
			Mode: ModeBlob,
			Name: fileName,
			Hash: blobHash,
		})
	}

	treeBytes := SerializeTree(entries)
	treeObj := NewObject(TypeTree, treeBytes)
	_, err := s.Put(treeObj.RawData)
	if err != nil {
		return "", fmt.Errorf("failed to save tree object: %w", err)
	}
	return treeObj.Hash, nil
}

// FlattenTree recursively traverses a tree object and returns a map of (relPath -> blobHash).
func FlattenTree(treeHash string, s Store) (map[string]string, error) {
	files := make(map[string]string)
	err := flattenTreeRecursive(treeHash, "", s, files)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func flattenTreeRecursive(treeHash, prefix string, s Store, files map[string]string) error {
	raw, err := s.Get(treeHash)
	if err != nil {
		return fmt.Errorf("get tree object %s: %w", treeHash, err)
	}
	obj, err := ParseRaw(raw)
	if err != nil {
		return fmt.Errorf("parse tree object %s: %w", treeHash, err)
	}
	if obj.Type != TypeTree {
		return fmt.Errorf("object %s is %s, expected tree", treeHash, obj.Type)
	}
	tree, err := ParseTree(obj.Data)
	if err != nil {
		return fmt.Errorf("parse tree entries for %s: %w", treeHash, err)
	}

	for _, entry := range tree.Entries {
		currPath := entry.Name
		if prefix != "" {
			currPath = prefix + "/" + entry.Name
		}
		if entry.Mode == ModeTree {
			if err := flattenTreeRecursive(entry.Hash, currPath, s, files); err != nil {
				return err
			}
		} else {
			files[currPath] = entry.Hash
		}
	}
	return nil
}
