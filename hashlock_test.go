package hashlock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateAndVerifyRoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dir", "b.txt"), []byte("beta"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.tmp"), []byte("temporary"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := Generate(root, GenerateOptions{
		Excludes: []string{"*.tmp"},
		Now:      func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != Version {
		t.Fatalf("unexpected version: %d", manifest.Version)
	}
	if len(manifest.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(manifest.Files))
	}

	report, err := Verify(root, manifest, VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.HasProblems() {
		t.Fatalf("unexpected problems: %+v", report)
	}
}

func TestVerificationDetectsChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dir", "b.txt"), []byte("beta"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := Generate(root, GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("ALPHA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "dir", "b.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "extra.txt"), []byte("extra"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Verify(root, manifest, VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Modified) != 1 || report.Modified[0].Path != "a.txt" {
		t.Fatalf("unexpected modified files: %+v", report.Modified)
	}
	if len(report.Missing) != 1 || report.Missing[0] != "dir/b.txt" {
		t.Fatalf("unexpected missing files: %v", report.Missing)
	}
	if len(report.Extra) != 1 || report.Extra[0] != "extra.txt" {
		t.Fatalf("unexpected extra files: %v", report.Extra)
	}
}

func TestManifestValidationRejectsInvalidExcludePattern(t *testing.T) {
	manifest := Manifest{
		Version:   Version,
		Algorithm: "sha256",
		CreatedAt: time.Now().UTC(),
		Files: []FileEntry{
			{Path: "a.txt", Size: 5, SHA256: strings.Repeat("0", 64)},
		},
	}

	manifest.Excludes = []string{"["}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected invalid exclude pattern to be rejected")
	}
}

func TestManifestValidationAcceptsValidAndEmptyExcludePatterns(t *testing.T) {
	manifest := Manifest{
		Version:   Version,
		Algorithm: "sha256",
		CreatedAt: time.Now().UTC(),
		Files: []FileEntry{
			{Path: "a.txt", Size: 5, SHA256: strings.Repeat("0", 64)},
		},
		Excludes: []string{"*.tmp", "  ", ""},
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestGenerateRejectsInvalidExcludePattern(t *testing.T) {
	if _, err := Generate(t.TempDir(), GenerateOptions{Excludes: []string{"["}}); err == nil {
		t.Fatal("expected Generate to reject an invalid exclude pattern")
	}
}

func TestVerifyRejectsInvalidExcludeOverride(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := Generate(root, GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Verify(root, manifest, VerifyOptions{Excludes: []string{"["}}); err == nil {
		t.Fatal("expected Verify to reject an invalid exclude override")
	}
}

func TestManifestDiff(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("beta"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldManifest, err := Generate(root, GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("ALPHA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "b.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "c.txt"), []byte("gamma"), 0o644); err != nil {
		t.Fatal(err)
	}

	newManifest, err := Generate(root, GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	diff, err := Diff(oldManifest, newManifest)
	if err != nil {
		t.Fatal(err)
	}
	if !diff.HasChanges() {
		t.Fatal("expected diff to report changes")
	}
	if len(diff.Added) != 1 || diff.Added[0] != "c.txt" {
		t.Fatalf("unexpected added files: %v", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0] != "b.txt" {
		t.Fatalf("unexpected removed files: %v", diff.Removed)
	}
	if len(diff.Changed) != 1 || diff.Changed[0].Path != "a.txt" {
		t.Fatalf("unexpected changed files: %+v", diff.Changed)
	}

	identical, err := Diff(oldManifest, oldManifest)
	if err != nil {
		t.Fatal(err)
	}
	if identical.HasChanges() {
		t.Fatalf("expected identical manifests to produce no changes: %+v", identical)
	}
}

func TestGenerateExcludesTrailingSlashDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tmpdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tmpdir", "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := Generate(root, GenerateOptions{Excludes: []string{"tmpdir/"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Path != "keep.txt" {
		t.Fatalf("unexpected files: %+v", manifest.Files)
	}
}
