# hashlock

`hashlock` is a small, dependency-free Go CLI and library for generating and verifying SHA-256 manifests for directory trees. It is useful for release artifacts, backups, static sites, and any workflow where you want to detect missing, changed, or unexpected files.

## Features

- Recursively hashes regular files with SHA-256.
- Writes deterministic JSON manifests sorted by relative path.
- Verifies a directory against a manifest.
- Reports missing, modified, and extra files.
- Supports comma-separated glob exclusion patterns.
- Provides both a command-line tool and a Go library.

## Build

```sh
go build ./cmd/hashlock
```

## Usage

Generate a manifest:

```sh
hashlock generate ./site -o site.manifest.json
```

Generate a manifest while excluding temporary files:

```sh
hashlock generate ./site --exclude '*.tmp,*.log' -o site.manifest.json
```

Verify a directory:

```sh
hashlock verify ./site site.manifest.json
```

Print a machine-readable verification report:

```sh
hashlock verify --json ./site site.manifest.json
```

## Manifest format

```json
{
  "version": 1,
  "algorithm": "sha256",
  "created_at": "2025-01-01T00:00:00Z",
  "excludes": ["*.tmp"],
  "files": [
    {
      "path": "index.html",
      "size": 123,
      "sha256": "..."
    }
  ]
}
```

## Library example

```go
package main

import (
    "fmt"

    "github.com/atlas7649/hashlock"
)

func main() {
    manifest, err := hashlock.Generate("./site", hashlock.GenerateOptions{})
    if err != nil {
        panic(err)
    }

    report, err := hashlock.Verify("./site", manifest, hashlock.VerifyOptions{})
    if err != nil {
        panic(err)
    }

    if report.HasProblems() {
        fmt.Printf("missing=%d modified=%d extra=%d\n",
            len(report.Missing), len(report.Modified), len(report.Extra))
    } else {
        fmt.Println("directory matches manifest")
    }
}
```

## Exit codes

- `0`: success, or verification found no problems.
- `1`: verification found missing, modified, or extra files.
- `2`: usage error or runtime error.

## Development

```sh
go test ./...
```
