package hashlock

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func scan(root string, excludes []string) ([]FileEntry, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	var entries []FileEntry
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		slashPath := filepath.ToSlash(rel)

		if d.IsDir() {
			if matchAny(excludes, slashPath) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			// Symlinks and special files are intentionally not followed.
			return nil
		}
		if matchAny(excludes, slashPath) {
			return nil
		}

		entry, err := hashFile(path, slashPath)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	if entries == nil {
		entries = []FileEntry{}
	}
	return entries, nil
}

func hashFile(path, relativePath string) (FileEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return FileEntry{}, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return FileEntry{}, err
	}

	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return FileEntry{}, err
	}
	if size != stat.Size() {
		return FileEntry{}, fmt.Errorf("file changed while hashing: %s", relativePath)
	}

	return FileEntry{
		Path:   relativePath,
		Size:   size,
		SHA256: hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

func matchAny(patterns []string, candidate string) bool {
	for _, rawPattern := range patterns {
		pattern := strings.TrimSpace(rawPattern)
		if pattern == "" {
			continue
		}
		if ok, err := filepath.Match(pattern, candidate); err == nil && ok {
			return true
		}
		if !strings.Contains(pattern, "/") {
			if ok, err := filepath.Match(pattern, filepath.Base(candidate)); err == nil && ok {
				return true
			}
		}
	}
	return false
}
