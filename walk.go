package hashlock

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func scan(root string, excludes []string) ([]FileEntry, error) {
	cleanRoot := filepath.Clean(root)
	info, err := os.Lstat(cleanRoot)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	var entries []FileEntry
	err = filepath.WalkDir(cleanRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == cleanRoot {
			return nil
		}

		rel, err := filepath.Rel(cleanRoot, path)
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
		if matchFile(excludes, slashPath) {
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
		// Relative manifest paths never end in a slash, so a trailing slash in
		// an exclusion pattern is only a user-friendly way to denote a directory.
		pattern = strings.TrimRight(pattern, "/")
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if ok, err := path.Match(pattern, candidate); err == nil && ok {
			return true
		}
		if !strings.Contains(pattern, "/") {
			if ok, err := path.Match(pattern, filepath.Base(candidate)); err == nil && ok {
				return true
			}
		}
	}
	return false
}

// matchFile applies exclusion patterns to regular files. A trailing slash
// marks a directory-only pattern, so such patterns never exclude files.
func matchFile(patterns []string, candidate string) bool {
	for _, rawPattern := range patterns {
		pattern := strings.TrimSpace(rawPattern)
		if pattern == "" || strings.HasSuffix(pattern, "/") {
			continue
		}
		if ok, err := path.Match(pattern, candidate); err == nil && ok {
			return true
		}
		if !strings.Contains(pattern, "/") {
			if ok, err := path.Match(pattern, filepath.Base(candidate)); err == nil && ok {
				return true
			}
		}
	}
	return false
}
