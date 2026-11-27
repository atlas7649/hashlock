package main

import (
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/atlas7649/hashlock"
)

func main() {
    os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
    if len(args) == 0 {
        printUsage(os.Stderr)
        return 2
    }

    switch args[0] {
    case "generate":
        return runGenerate(args[1:])
    case "verify":
        return runVerify(args[1:])
    case "audit":
        return runAudit(args[1:])
    case "diff":
        return runDiff(args[1:])
    case "help", "-h", "--help":
        printUsage(os.Stdout)
        return 0
    default:
        fmt.Fprintf(os.Stderr, "hashlock: unknown command %q\n", args[0])
        printUsage(os.Stderr)
        return 2
    }
}

func runGenerate(args []string) int {
    var output, excludes string

    fs := flag.NewFlagSet("generate", flag.ContinueOnError)
    fs.SetOutput(os.Stderr)
    fs.StringVar(&output, "o", "-", "output file (use - for stdout)")
    fs.StringVar(&output, "output", "-", "output file (use - for stdout)")
    fs.StringVar(&excludes, "exclude", "", "comma-separated glob exclusion patterns")
    fs.StringVar(&excludes, "excludes", "", "comma-separated glob exclusion patterns")
    fs.Usage = func() {
        fmt.Fprintln(os.Stderr, "Usage: hashlock generate [options] <directory>")
        fmt.Fprintln(os.Stderr)
        fmt.Fprintln(os.Stderr, "Options:")
        fs.PrintDefaults()
    }

    args = reorderArgs(fs, args)
    if err := fs.Parse(args); err != nil {
        if errors.Is(err, flag.ErrHelp) {
            return 0
        }
        fmt.Fprintf(os.Stderr, "hashlock generate: %v\n", err)
        return 2
    }

    positional := fs.Args()
    if len(positional) != 1 {
        fmt.Fprintln(os.Stderr, "hashlock generate: expected exactly one directory argument")
        fs.Usage()
        return 2
    }

    manifest, err := hashlock.Generate(positional[0], hashlock.GenerateOptions{
        Excludes: splitExcludes(excludes),
    })
    if err != nil {
        fmt.Fprintf(os.Stderr, "hashlock generate: %v\n", err)
        return 2
    }

    data, err := json.MarshalIndent(manifest, "", "  ")
    if err != nil {
        fmt.Fprintf(os.Stderr, "hashlock generate: %v\n", err)
        return 2
    }
    data = append(data, '\n')

    if output == "-" {
        if _, err := os.Stdout.Write(data); err != nil {
            fmt.Fprintf(os.Stderr, "hashlock generate: %v\n", err)
            return 2
        }
        return 0
    }

    if err := os.WriteFile(output, data, 0o644); err != nil {
        fmt.Fprintf(os.Stderr, "hashlock generate: %v\n", err)
        return 2
    }
    return 0
}

func runVerify(args []string) int {
    var jsonOutput bool
    var excludes string

    fs := flag.NewFlagSet("verify", flag.ContinueOnError)
    fs.SetOutput(os.Stderr)
    fs.BoolVar(&jsonOutput, "json", false, "print a machine-readable verification report")
    fs.StringVar(&excludes, "exclude", "", "comma-separated glob exclusion patterns")
    fs.StringVar(&excludes, "excludes", "", "comma-separated glob exclusion patterns")
    fs.Usage = func() {
        fmt.Fprintln(os.Stderr, "Usage: hashlock verify [options] <directory> <manifest>")
        fmt.Fprintln(os.Stderr)
        fmt.Fprintln(os.Stderr, "Options:")
        fs.PrintDefaults()
    }

    args = reorderArgs(fs, args)
    if err := fs.Parse(args); err != nil {
        if errors.Is(err, flag.ErrHelp) {
            return 0
        }
        fmt.Fprintf(os.Stderr, "hashlock verify: %v\n", err)
        return 2
    }

    positional := fs.Args()
    if len(positional) != 2 {
        fmt.Fprintln(os.Stderr, "hashlock verify: expected a directory and a manifest")
        fs.Usage()
        return 2
    }

    manifest, err := readManifest(positional[1])
    if err != nil {
        fmt.Fprintf(os.Stderr, "hashlock verify: %v\n", err)
        return 2
    }

    opts := hashlock.VerifyOptions{}
    if excludes != "" {
        opts.Excludes = splitExcludes(excludes)
    }

    report, err := hashlock.Verify(positional[0], manifest, opts)
    if err != nil {
        fmt.Fprintf(os.Stderr, "hashlock verify: %v\n", err)
        return 2
    }

    if jsonOutput {
        data, err := json.MarshalIndent(report, "", "  ")
        if err != nil {
            fmt.Fprintf(os.Stderr, "hashlock verify: %v\n", err)
            return 2
        }
        data = append(data, '\n')
        if _, err := os.Stdout.Write(data); err != nil {
            fmt.Fprintf(os.Stderr, "hashlock verify: %v\n", err)
            return 2
        }
    } else {
        printVerification(report)
    }

    if report.HasProblems() {
        return 1
    }
    return 0
}

func runAudit(args []string) int {
    fs := flag.NewFlagSet("audit", flag.ContinueOnError)
    fs.SetOutput(os.Stderr)
    fs.Usage = func() {
        fmt.Fprintln(os.Stderr, "Usage: hashlock audit [options] <manifest>")
        fmt.Fprintln(os.Stderr)
        fmt.Fprintln(os.Stderr, "Options:")
        fs.PrintDefaults()
    }

    args = reorderArgs(fs, args)
    if err := fs.Parse(args); err != nil {
        if errors.Is(err, flag.ErrHelp) {
            return 0
        }
        fmt.Fprintf(os.Stderr, "hashlock audit: %v\n", err)
        return 2
    }

    positional := fs.Args()
    if len(positional) != 1 {
        fmt.Fprintln(os.Stderr, "hashlock audit: expected exactly one manifest argument")
        fs.Usage()
        return 2
    }

    manifest, err := readManifest(positional[0])
    if err != nil {
        fmt.Fprintf(os.Stderr, "hashlock audit: %v\n", err)
        return 2
    }

    if err := manifest.Validate(); err != nil {
        fmt.Fprintf(os.Stderr, "hashlock audit: %v\n", err)
        return 1
    }

    fmt.Println("audit: OK")
    return 0
}

func runDiff(args []string) int {
    var jsonOutput bool

    fs := flag.NewFlagSet("diff", flag.ContinueOnError)
    fs.SetOutput(os.Stderr)
    fs.BoolVar(&jsonOutput, "json", false, "print a machine-readable diff")
    fs.Usage = func() {
        fmt.Fprintln(os.Stderr, "Usage: hashlock diff [options] <old-manifest> <new-manifest>")
        fmt.Fprintln(os.Stderr)
        fmt.Fprintln(os.Stderr, "Options:")
        fs.PrintDefaults()
    }

    args = reorderArgs(fs, args)
    if err := fs.Parse(args); err != nil {
        if errors.Is(err, flag.ErrHelp) {
            return 0
        }
        fmt.Fprintf(os.Stderr, "hashlock diff: %v\n", err)
        return 2
    }

    positional := fs.Args()
    if len(positional) != 2 {
        fmt.Fprintln(os.Stderr, "hashlock diff: expected two manifest arguments")
        fs.Usage()
        return 2
    }

    oldManifest, err := readManifest(positional[0])
    if err != nil {
        fmt.Fprintf(os.Stderr, "hashlock diff: %v\n", err)
        return 2
    }
    newManifest, err := readManifest(positional[1])
    if err != nil {
        fmt.Fprintf(os.Stderr, "hashlock diff: %v\n", err)
        return 2
    }

    diff, err := hashlock.Diff(oldManifest, newManifest)
    if err != nil {
        fmt.Fprintf(os.Stderr, "hashlock diff: %v\n", err)
        return 2
    }

    if jsonOutput {
        data, err := json.MarshalIndent(diff, "", "  ")
        if err != nil {
            fmt.Fprintf(os.Stderr, "hashlock diff: %v\n", err)
            return 2
        }
        data = append(data, '\n')
        if _, err := os.Stdout.Write(data); err != nil {
            fmt.Fprintf(os.Stderr, "hashlock diff: %v\n", err)
            return 2
        }
    } else {
        printDiff(diff)
    }

    if diff.HasChanges() {
        return 1
    }
    return 0
}

func readManifest(name string) (hashlock.Manifest, error) {
    var data []byte
    var err error

    if name == "-" {
        data, err = io.ReadAll(os.Stdin)
    } else {
        data, err = os.ReadFile(name)
    }
    if err != nil {
        return hashlock.Manifest{}, err
    }

    var manifest hashlock.Manifest
    if err := json.Unmarshal(data, &manifest); err != nil {
        return hashlock.Manifest{}, err
    }
    return manifest, nil
}

func splitExcludes(value string) []string {
    if value == "" {
        return nil
    }
    return strings.Split(value, ",")
}

func printVerification(report hashlock.Verification) {
    fmt.Printf("ok: %d\n", len(report.OK))
    fmt.Printf("missing: %d\n", len(report.Missing))
    for _, path := range report.Missing {
        fmt.Printf("  %s\n", path)
    }
    fmt.Printf("modified: %d\n", len(report.Modified))
    for _, mismatch := range report.Modified {
        fmt.Printf("  %s\n", mismatch.Path)
    }
    fmt.Printf("extra: %d\n", len(report.Extra))
    for _, path := range report.Extra {
        fmt.Printf("  %s\n", path)
    }
}

func printDiff(diff hashlock.ManifestDiff) {
    if !diff.HasChanges() {
        fmt.Println("no changes")
        return
    }

    fmt.Printf("added: %d\n", len(diff.Added))
    for _, path := range diff.Added {
        fmt.Printf("  %s\n", path)
    }
    fmt.Printf("removed: %d\n", len(diff.Removed))
    for _, path := range diff.Removed {
        fmt.Printf("  %s\n", path)
    }
    fmt.Printf("changed: %d\n", len(diff.Changed))
    for _, change := range diff.Changed {
        fmt.Printf("  %s\n", change.Path)
    }
}

func printUsage(w io.Writer) {
    fmt.Fprintln(w, "Usage: hashlock <command> [options] [arguments]")
    fmt.Fprintln(w)
    fmt.Fprintln(w, "Commands:")
    fmt.Fprintln(w, "  generate  Generate a SHA-256 manifest for a directory")
    fmt.Fprintln(w, "  verify    Verify a directory against a manifest")
    fmt.Fprintln(w, "  audit     Audit a manifest's structure and hashes")
    fmt.Fprintln(w, "  diff      Compare two manifests")
    fmt.Fprintln(w)
    fmt.Fprintln(w, "Use \"hashlock <command> -h\" for command-specific help.")
}

func reorderArgs(fs *flag.FlagSet, args []string) []string {
    var flags []string
    var positionals []string

    i := 0
    for i < len(args) {
        arg := args[i]
        if arg == "--" {
            positionals = append(positionals, args[i:]...)
            break
        }
        if !strings.HasPrefix(arg, "-") || arg == "-" {
            positionals = append(positionals, arg)
            i++
            continue
        }

        name := arg
        inlineValue := false
        if index := strings.IndexByte(arg, '='); index >= 0 {
            name = arg[:index]
            inlineValue = true
        }
        name = strings.TrimLeft(name, "-")

        flagDef := fs.Lookup(name)
        isBool := false
        if flagDef != nil {
            if value, ok := flagDef.Value.(interface{ IsBoolFlag() bool }); ok && value.IsBoolFlag() {
                isBool = true
            }
        }

        if inlineValue || isBool || flagDef == nil {
            flags = append(flags, arg)
            i++
            if flagDef == nil && !inlineValue && i < len(args) && !strings.HasPrefix(args[i], "-") {
                // Preserve a value for unknown flags that are not boolean flags.
                flags = append(flags, args[i])
                i++
            }
            continue
        }

        flags = append(flags, arg)
        if i+1 < len(args) {
            flags = append(flags, args[i+1])
            i += 2
        } else {
            i++
        }
    }

    return append(flags, positionals...)
}
