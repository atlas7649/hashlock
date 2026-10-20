package hashlock

import (
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
)

// Verify compares root against manifest and returns a detailed report.
func Verify(root string, manifest Manifest, opts VerifyOptions) (Verification, error) {
	if err := manifest.Validate(); err != nil {
		return Verification{}, err
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

// Validate checks that a manifest is internally consistent.
func (m *Manifest) Validate() error {
	if m.Version != Version {
		return fmt.Errorf("unsupported manifest version %d", m.Version)
	}
	if m.Algorithm != "sha256" {
		return fmt.Errorf("unsupported algorithm %q", m.Algorithm)
	}

	seen := make(map[string]struct{}, len(m.Files))
	for _, entry := range m.Files {
		if entry.Path == "" {
			return errors.New("manifest contains an empty path")
		}
		clean := filepath.ToSlash(filepath.Clean(entry.Path))
		if clean != entry.Path || entry.Path == "." || filepath.IsAbs(entry.Path) {
			return fmt.Errorf("invalid manifest path %q", entry.Path)
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
