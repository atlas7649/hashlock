package hashlock

import (
	"time"
)

const Version = 1

// GenerateOptions controls manifest generation.
type GenerateOptions struct {
	// Excludes is a list of glob patterns. Each pattern is matched against
	// slash-separated relative paths.
	Excludes []string

	// Now optionally overrides the creation timestamp, which is useful for
	// deterministic tests.
	Now func() time.Time
}

// VerifyOptions controls verification.
type VerifyOptions struct {
	// Excludes overrides the excludes stored in the manifest. If nil, the
	// manifest's excludes are used.
	Excludes []string
}

// FileEntry describes one hashed file.
type FileEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Manifest is the serialized directory manifest.
type Manifest struct {
	Version   int         `json:"version"`
	Algorithm string      `json:"algorithm"`
	CreatedAt time.Time   `json:"created_at"`
	Excludes  []string    `json:"excludes,omitempty"`
	Files     []FileEntry `json:"files"`
}

// Verification contains the result of comparing a directory to a manifest.
type Verification struct {
	OK       []string       `json:"ok"`
	Missing  []string       `json:"missing"`
	Modified []FileMismatch `json:"modified"`
	Extra    []string       `json:"extra"`
}

// FileMismatch describes a file whose size or hash differs.
type FileMismatch struct {
	Path           string `json:"path"`
	ExpectedSHA256 string `json:"expected_sha256"`
	ActualSHA256   string `json:"actual_sha256"`
}

// HasProblems reports whether verification found any discrepancy.
func (v Verification) HasProblems() bool {
	return len(v.Missing) > 0 || len(v.Modified) > 0 || len(v.Extra) > 0
}

// Generate walks root and returns a manifest containing every regular file.
func Generate(root string, opts GenerateOptions) (Manifest, error) {
	entries, err := scan(root, opts.Excludes)
	if err != nil {
		return Manifest{}, err
	}

	now := time.Now().UTC()
	if opts.Now != nil {
		now = opts.Now().UTC()
	}

	excludes := append([]string(nil), opts.Excludes...)
	if len(excludes) == 0 {
		excludes = nil
	}

	manifest := Manifest{
		Version:   Version,
		Algorithm: "sha256",
		CreatedAt: now,
		Excludes:  excludes,
		Files:     entries,
	}
	if manifest.Files == nil {
		manifest.Files = []FileEntry{}
	}
	return manifest, nil
}
