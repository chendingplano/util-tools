// Command typst-ref extracts References-section citations out of a Typst
// file, files them into a references.bib, and rewrites in-text citations to
// Typst's @key syntax. It parses flags, wires I/O and sets exit codes; all
// logic lives in internal/refs.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/chendingplano/util-tools/typst-ref/internal/refs"
)

const descriptor = `{
  "name": "typst-ref",
  "summary": "Extract a Typst file's References section into references.bib and rewrite citations to @key form",
  "keywords": ["typst", "typ", "references", "bibliography", "bib", "citations", "knowledgestore"],
  "args": [
    {"name": "file", "type": "path", "required": true, "help": "Typst (.typ) file to process in place"},
    {"name": "--bib", "type": "path", "required": false, "help": "Path to references.bib; defaults to the nearest references/references.bib found walking up from file"},
    {"name": "--json", "type": "bool", "help": "Emit a JSON report instead of text"},
    {"name": "--dry-run", "type": "bool", "help": "Report what would change without writing any file"}
  ],
  "examples": ["typst-ref KnowledgeStore/Research/Notes.typ", "typst-ref --dry-run --json KnowledgeStore/Research/Notes.typ"]
}`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type jsonReference struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type jsonReport struct {
	File              string          `json:"file"`
	Bib               string          `json:"bib"`
	DryRun            bool            `json:"dry_run"`
	SectionFound      bool            `json:"section_found"`
	SectionRemoved    bool            `json:"section_removed"`
	Added             []jsonReference `json:"added"`
	AlreadyPresent    []jsonReference `json:"already_present"`
	Replaced          int             `json:"replaced"`
	BibliographyAdded bool            `json:"bibliography_added"`
	Warnings          []string        `json:"warnings"`
}

func toJSONRefs(rs []refs.Reference) []jsonReference {
	out := make([]jsonReference, len(rs))
	for i, r := range rs {
		out[i] = jsonReference{Slug: r.Slug, Title: r.Title, URL: r.URL}
	}
	return out
}

// run executes the tool. It returns 0 on success, 1 on a tool error and 2 on
// a usage error.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("typst-ref", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit a JSON report instead of text")
	dryRun := fs.Bool("dry-run", false, "report what would change without writing any file")
	bibFlag := fs.String("bib", "", "path to references.bib (default: nearest references/references.bib found walking up from file)")
	describe := fs.Bool("describe", false, "print this tool's JSON descriptor and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *describe {
		fmt.Fprintln(stdout, descriptor)
		return 0
	}

	if fs.NArg() != 1 || fs.Arg(0) == "-" {
		fmt.Fprintln(stderr, "typst-ref: exactly one Typst file path is required (in-place edits need a real path, not stdin)")
		return 2
	}
	typPath := fs.Arg(0)

	typBytes, err := os.ReadFile(typPath)
	if err != nil {
		fmt.Fprintf(stderr, "typst-ref: %v\n", err)
		return 1
	}

	bibPath := *bibFlag
	if bibPath == "" {
		bibPath, err = findBibPath(typPath)
		if err != nil {
			fmt.Fprintf(stderr, "typst-ref: %v\n", err)
			return 1
		}
	}

	var bibBytes []byte
	if b, err := os.ReadFile(bibPath); err == nil {
		bibBytes = b
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "typst-ref: %v\n", err)
		return 1
	}

	newTyp, newBib, report, err := refs.Process(string(typBytes), string(bibBytes))
	if err != nil {
		fmt.Fprintf(stderr, "typst-ref: %v\n", err)
		return 1
	}

	bibliographyAdded := false
	if report.Replaced > 0 {
		relBib, err := relBibPath(typPath, bibPath)
		if err != nil {
			fmt.Fprintf(stderr, "typst-ref: %v\n", err)
			return 1
		}
		newTyp, bibliographyAdded = refs.EnsureBibliography(newTyp, relBib)
	}

	if !*dryRun {
		if len(report.Added) > 0 {
			if err := writeFileAtomic(bibPath, newBib); err != nil {
				fmt.Fprintf(stderr, "typst-ref: %v\n", err)
				return 1
			}
		}
		if newTyp != string(typBytes) {
			if err := writeFileAtomic(typPath, newTyp); err != nil {
				fmt.Fprintf(stderr, "typst-ref: %v\n", err)
				return 1
			}
		}
	}

	if *asJSON {
		out := jsonReport{
			File:              typPath,
			Bib:               bibPath,
			DryRun:            *dryRun,
			SectionFound:      report.SectionFound,
			SectionRemoved:    report.SectionRemoved,
			Added:             toJSONRefs(report.Added),
			AlreadyPresent:    toJSONRefs(report.AlreadyPresent),
			Replaced:          report.Replaced,
			BibliographyAdded: bibliographyAdded,
			Warnings:          report.Warnings,
		}
		if out.Added == nil {
			out.Added = []jsonReference{}
		}
		if out.AlreadyPresent == nil {
			out.AlreadyPresent = []jsonReference{}
		}
		if out.Warnings == nil {
			out.Warnings = []string{}
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(stderr, "typst-ref: %v\n", err)
			return 1
		}
		return 0
	}

	printReport(stdout, stderr, typPath, bibPath, report, bibliographyAdded, *dryRun)
	return 0
}

func printReport(stdout, stderr io.Writer, typPath, bibPath string, report refs.Report, bibliographyAdded, dryRun bool) {
	prefix := ""
	if dryRun {
		prefix = "[dry-run] "
	}
	if !report.SectionFound {
		fmt.Fprintf(stdout, "%sno References section found in %s\n", prefix, typPath)
		return
	}
	for _, r := range report.Added {
		fmt.Fprintf(stdout, "%sadded %s (%s) to %s\n", prefix, r.Slug, r.URL, bibPath)
	}
	for _, r := range report.AlreadyPresent {
		fmt.Fprintf(stdout, "%salready in %s: %s\n", prefix, bibPath, r.Slug)
	}
	if report.Replaced > 0 {
		fmt.Fprintf(stdout, "%sreplaced %d citation(s) in %s\n", prefix, report.Replaced, typPath)
	}
	if report.SectionRemoved {
		fmt.Fprintf(stdout, "%sremoved References section from %s\n", prefix, typPath)
	}
	if bibliographyAdded {
		fmt.Fprintf(stdout, "%sadded #bibliography(...) call to %s\n", prefix, typPath)
	}
	for _, w := range report.Warnings {
		fmt.Fprintf(stderr, "typst-ref: warning: %s\n", w)
	}
}

// findBibPath walks up from the directory containing typPath looking for a
// "references" subdirectory (the marker for a KnowledgeStore-style root) and
// returns the path to references.bib inside it.
func findBibPath(typPath string) (string, error) {
	dir, err := filepath.Abs(filepath.Dir(typPath))
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "references")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return filepath.Join(candidate, "references.bib"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no references/ directory found walking up from %s; pass --bib explicitly", filepath.Dir(typPath))
		}
		dir = parent
	}
}

// relBibPath returns bibPath relative to typPath's directory, in Typst's
// forward-slash path form, for use in a #bibliography(...) call.
func relBibPath(typPath, bibPath string) (string, error) {
	typDir, err := filepath.Abs(filepath.Dir(typPath))
	if err != nil {
		return "", err
	}
	absBib, err := filepath.Abs(bibPath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(typDir, absBib)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// writeFileAtomic writes content to path by writing a temp file in the same
// directory and renaming it into place, so a failed write never leaves a
// partial file behind.
func writeFileAtomic(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".typst-ref-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
