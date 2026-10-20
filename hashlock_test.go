package hashlock

import (
	"os"
	"path/filepath"
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
