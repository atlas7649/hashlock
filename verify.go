package hashlock

import (
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
)

// Verify compares root against manifest and returns a detailed report.
func Verify(root string, manifest Manifest, opts VerifyOptions) (Verification, error) {
	if err := manifest.Validate(); err != nil {
		return Verification{}, err
	}

	if opts.Excludes != nil {
		if err := validateExcludes(opts.Excludes); err != nil {
			return Verification{}, err
		}
	}

	excludes := opts.Excludes
	if excludes == nil {
		excludes = manifest.Excludes
	}

	current, err := scan(root, excludes)
	if err != nil {
		return Verification{}, err
	}

	currentMap := make(map[string]FileEntry, len(current))
	for _, entry := range current {
		currentMap[entry.Path] = entry
	}
	manifestMap := make(map[string]FileEntry, len(manifest.Files))
	for _, entry := range manifest.Files {
		manifestMap[entry.Path] = entry
	}

	report := Verification{
		OK:       []string{},
		Missing:  []string{},
		Modified: []FileMismatch{},
		Extra:    []string{},
	}

	for _, expected := range manifest.Files {
		actual, ok := currentMap[expected.Path]
		if !ok {
			report.Missing = append(report.Missing, expected.Path)
			continue
		}
		if actual.Size != expected.Size || actual.SHA256 != expected.SHA256 {
			report.Modified = append(report.Modified, FileMismatch{
				Path:           expected.Path,
				ExpectedSHA256: expected.SHA256,
				ActualSHA256:   actual.SHA256,
			})
		} else {
			report.OK = append(report.OK, expected.Path)
		}
	}

	for _, actual := range current {
		if _, ok := manifestMap[actual.Path]; !ok {
			report.Extra = append(report.Extra, actual.Path)
		}
	}

	sort.Strings(report.OK)
	sort.Strings(report.Missing)
	sort.Strings(report.Extra)
	sort.Slice(report.Modified, func(i, j int) bool {
		return report.Modified[i].Path < report.Modified[j].Path
	})

	return report, nil
}

// Diff compares two manifests and reports added, removed, and changed files.
func Diff(old, new Manifest) (ManifestDiff, error) {
	if err := old.Validate(); err != nil {
		return ManifestDiff{}, err
	}
	if err := new.Validate(); err != nil {
		return ManifestDiff{}, err
	}

	oldMap := make(map[string]FileEntry, len(old.Files))
	for _, entry := range old.Files {
		oldMap[entry.Path] = entry
	}
	newMap := make(map[string]FileEntry, len(new.Files))
	for _, entry := range new.Files {
		newMap[entry.Path] = entry
	}

	diff := ManifestDiff{
		Added:   []string{},
		Removed: []string{},
		Changed: []FileChange{},
	}

	for _, entry := range old.Files {
		if current, ok := newMap[entry.Path]; ok {
			if current.Size != entry.Size || current.SHA256 != entry.SHA256 {
				diff.Changed = append(diff.Changed, FileChange{
					Path:      entry.Path,
					OldSize:   entry.Size,
					NewSize:   current.Size,
					OldSHA256: entry.SHA256,
					NewSHA256: current.SHA256,
				})
			}
		} else {
			diff.Removed = append(diff.Removed, entry.Path)
		}
	}

	for _, entry := range new.Files {
		if _, ok := oldMap[entry.Path]; !ok {
			diff.Added = append(diff.Added, entry.Path)
		}
	}

	sort.Strings(diff.Added)
	sort.Strings(diff.Removed)
	sort.Slice(diff.Changed, func(i, j int) bool {
		return diff.Changed[i].Path < diff.Changed[j].Path
	})

	return diff, nil
}

// Validate checks that a manifest is internally consistent.
func (m *Manifest) Validate() error {
	if m.Version != Version {
		return fmt.Errorf("unsupported manifest version %d", m.Version)
	}
	if m.Algorithm != "sha256" {
		return fmt.Errorf("unsupported algorithm %q", m.Algorithm)
	}

	for _, rawPattern := range m.Excludes {
		pattern := strings.TrimSpace(rawPattern)
		if pattern == "" {
			continue
		}
		if _, err := path.Match(pattern, ""); err != nil {
			return fmt.Errorf("invalid exclude pattern %q: %w", rawPattern, err)
		}
	}

	seen := make(map[string]struct{}, len(m.Files))
	for _, entry := range m.Files {
		if entry.Path == "" {
			return errors.New("manifest contains an empty path")
		}
		clean := path.Clean(entry.Path)
		if clean != entry.Path || entry.Path == "." || entry.Path == "/" || path.IsAbs(entry.Path) {
			return fmt.Errorf("invalid manifest path %q", entry.Path)
		}
		for _, part := range strings.Split(entry.Path, "/") {
			if part == ".." {
				return fmt.Errorf("invalid manifest path %q", entry.Path)
			}
		}
		if _, ok := seen[entry.Path]; ok {
			return fmt.Errorf("duplicate manifest path %q", entry.Path)
		}
		seen[entry.Path] = struct{}{}

		if len(entry.SHA256) != 64 {
			return fmt.Errorf("invalid sha256 length for %q", entry.Path)
		}
		if _, err := hex.DecodeString(entry.SHA256); err != nil {
			return fmt.Errorf("invalid sha256 for %q: %w", entry.Path, err)
		}
		if entry.Size < 0 {
			return fmt.Errorf("negative size for %q", entry.Path)
		}
	}
	return nil
}

func validateExcludes(patterns []string) error {
	for _, rawPattern := range patterns {
		pattern := strings.TrimSpace(rawPattern)
		if pattern == "" {
			continue
		}
		if _, err := path.Match(pattern, ""); err != nil {
			return fmt.Errorf("invalid exclude pattern %q: %w", rawPattern, err)
		}
	}
	return nil
}
