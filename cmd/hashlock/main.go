package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/atlas7649/hashlock"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "generate":
		os.Exit(runGenerate(os.Args[2:]))
	case "verify":
		os.Exit(runVerify(os.Args[2:]))
	case "audit":
		os.Exit(runAudit(os.Args[2:]))
	case "version":
		fmt.Println("hashlock 0.1.0")
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: hashlock <generate|verify|audit|version> [options]")
	fmt.Fprintln(os.Stderr, "  hashlock generate [options] ROOT")
	fmt.Fprintln(os.Stderr, "  hashlock verify [options] ROOT MANIFEST|- ")
	fmt.Fprintln(os.Stderr, "  hashlock audit MANIFEST|- ")
}

func splitExcludes(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func runGenerate(args []string) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	output := fs.String("o", "-", "write manifest to FILE (- for stdout)")
	exclude := fs.String("exclude", "", "comma-separated glob patterns to exclude")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "generate requires exactly one ROOT argument")
		return 2
	}

	manifest, err := hashlock.Generate(fs.Arg(0), hashlock.GenerateOptions{
		Excludes: splitExcludes(*exclude),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "hashlock: %v\n", err)
		return 2
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "hashlock: %v\n", err)
		return 2
	}
	data = append(data, '\n')

	if *output == "-" {
		if _, err := os.Stdout.Write(data); err != nil {
			fmt.Fprintf(os.Stderr, "hashlock: %v\n", err)
			return 2
		}
		return 0
	}
	if err := os.WriteFile(*output, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "hashlock: %v\n", err)
		return 2
	}
	return 0
}

func runVerify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	exclude := fs.String("exclude", "", "comma-separated glob patterns to exclude")
	asJSON := fs.Bool("json", false, "print the verification report as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "verify requires ROOT and MANIFEST arguments")
		return 2
	}

	root := fs.Arg(0)
	manifestData, err := readManifest(fs.Arg(1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "hashlock: %v\n", err)
		return 2
	}
	var manifest hashlock.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "hashlock: invalid manifest: %v\n", err)
		return 2
	}

	report, err := hashlock.Verify(root, manifest, hashlock.VerifyOptions{
		Excludes: splitExcludes(*exclude),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "hashlock: %v\n", err)
		return 2
	}

	if *asJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "hashlock: %v\n", err)
			return 2
		}
		data = append(data, '\n')
		if _, err := os.Stdout.Write(data); err != nil {
			fmt.Fprintf(os.Stderr, "hashlock: %v\n", err)
			return 2
		}
		if report.HasProblems() {
			return 1
		}
		return 0
	}

	if report.HasProblems() {
		fmt.Printf("verification failed: %d missing, %d modified, %d extra\n",
			len(report.Missing), len(report.Modified), len(report.Extra))
		for _, path := range report.Missing {
			fmt.Printf("missing: %s\n", path)
		}
		for _, mismatch := range report.Modified {
			fmt.Printf("modified: %s\n", mismatch.Path)
		}
		for _, path := range report.Extra {
			fmt.Printf("extra: %s\n", path)
		}
		return 1
	}

	fmt.Println("verification passed")
	return 0
}

func runAudit(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "audit requires exactly one MANIFEST argument")
		return 2
	}
	manifestData, err := readManifest(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "hashlock: %v\n", err)
		return 2
	}
	var manifest hashlock.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "hashlock: invalid manifest: %v\n", err)
		return 2
	}
	if err := manifest.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "hashlock: audit failed: %v\n", err)
		return 1
	}
	fmt.Printf("manifest audit passed: %d files, algorithm %s\n", len(manifest.Files), manifest.Algorithm)
	return 0
}

func readManifest(name string) ([]byte, error) {
	if name == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read standard input: %w", err)
		}
		return data, nil
	}
	return os.ReadFile(name)
}
