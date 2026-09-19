package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupKnowledgeStore(t *testing.T) (typPath, bibPath string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	typPath = filepath.Join(root, "Notes.typ")
	content := `See ([Example][1]).

= References
[1]: https://example.com/page "Example Page"
`
	if err := os.WriteFile(typPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	bibPath = filepath.Join(root, "references", "references.bib")
	return typPath, bibPath
}

func TestRunWritesTypAndBib(t *testing.T) {
	typPath, bibPath := setupKnowledgeStore(t)

	var out, errOut bytes.Buffer
	code := run([]string{typPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}

	gotTyp, err := os.ReadFile(typPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(gotTyp), "= References") {
		t.Errorf("typ file still has References section:\n%s", gotTyp)
	}
	if !strings.Contains(string(gotTyp), "@example-page") {
		t.Errorf("typ file missing @example-page:\n%s", gotTyp)
	}

	gotBib, err := os.ReadFile(bibPath)
	if err != nil {
		t.Fatalf("bib file was not created: %v", err)
	}
	if !strings.Contains(string(gotBib), "@online{example-page,") {
		t.Errorf("bib file missing entry:\n%s", gotBib)
	}
	if !strings.Contains(string(gotTyp), `#bibliography("references/references.bib")`) {
		t.Errorf("typ file missing #bibliography(...) call so @example-page would fail to resolve:\n%s", gotTyp)
	}
}

func TestRunDoesNotDuplicateExistingBibliographyCall(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	typPath := filepath.Join(root, "Notes.typ")
	content := `See ([Example][1]).

= References
[1]: https://example.com/page "Example Page"

#bibliography("references/references.bib")
`
	if err := os.WriteFile(typPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := run([]string{typPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	gotTyp, err := os.ReadFile(typPath)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(gotTyp), "#bibliography("); n != 1 {
		t.Errorf("typ file has %d #bibliography( calls, want 1:\n%s", n, gotTyp)
	}
}

func TestRunDryRunWritesNothing(t *testing.T) {
	typPath, bibPath := setupKnowledgeStore(t)
	origTyp, _ := os.ReadFile(typPath)

	var out, errOut bytes.Buffer
	code := run([]string{"--dry-run", typPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}

	gotTyp, _ := os.ReadFile(typPath)
	if string(gotTyp) != string(origTyp) {
		t.Errorf("dry-run modified the typ file")
	}
	if _, err := os.Stat(bibPath); !os.IsNotExist(err) {
		t.Errorf("dry-run created the bib file")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("dry-run output missing marker: %q", out.String())
	}
}

func TestRunJSON(t *testing.T) {
	typPath, _ := setupKnowledgeStore(t)

	var out, errOut bytes.Buffer
	code := run([]string{"--json", typPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	var got jsonReport
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v (%q)", err, out.String())
	}
	if len(got.Added) != 1 || got.Added[0].Slug != "example-page" {
		t.Errorf("got.Added = %+v", got.Added)
	}
	if got.Replaced != 1 {
		t.Errorf("got.Replaced = %d, want 1", got.Replaced)
	}
}

func TestRunDescribe(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"--describe"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	var d struct {
		Name     string   `json:"name"`
		Summary  string   `json:"summary"`
		Keywords []string `json:"keywords"`
	}
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatalf("descriptor is not JSON: %v", err)
	}
	if d.Name != "typst-ref" {
		t.Errorf("descriptor name = %q, want %q", d.Name, "typst-ref")
	}
	if d.Summary == "" || len(d.Keywords) == 0 {
		t.Errorf("descriptor missing summary or keywords: %+v", d)
	}
}

func TestRunUsageErrorNoArgs(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{}, &out, &errOut)
	if code != 2 {
		t.Errorf("run() = %d, want 2 for missing file arg", code)
	}
}

func TestRunUsageErrorBadFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"--nope"}, &out, &errOut)
	if code != 2 {
		t.Errorf("run() = %d, want 2 for usage error", code)
	}
}

func TestRunToolErrorMissingFile(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{filepath.Join(t.TempDir(), "missing.typ")}, &out, &errOut)
	if code != 1 {
		t.Errorf("run() = %d, want 1 for missing file", code)
	}
}

func TestRunExplicitBibFlag(t *testing.T) {
	root := t.TempDir()
	typPath := filepath.Join(root, "Notes.typ")
	content := `([Example][1])

= References
[1]: https://example.com/page "Example Page"
`
	if err := os.WriteFile(typPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	bibPath := filepath.Join(root, "custom", "refs.bib")

	var out, errOut bytes.Buffer
	code := run([]string{"--bib", bibPath, typPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	if _, err := os.Stat(bibPath); err != nil {
		t.Errorf("explicit --bib path was not written: %v", err)
	}
}

func TestRunNoReferencesDirFound(t *testing.T) {
	root := t.TempDir()
	typPath := filepath.Join(root, "Notes.typ")
	if err := os.WriteFile(typPath, []byte("no refs here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No References section, so findBibPath is never consulted for writes,
	// but with a References section present and no references/ dir anywhere
	// above a system temp root, run should fail with a tool error.
	content := `([Example][1])

= References
[1]: https://example.com/page "Example Page"
`
	if err := os.WriteFile(typPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := run([]string{typPath}, &out, &errOut)
	if code != 1 {
		t.Errorf("run() = %d, want 1 when no references/ dir can be found; stderr=%q", code, errOut.String())
	}
}
