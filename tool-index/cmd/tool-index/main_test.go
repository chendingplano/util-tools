package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// harness writes a policy dir and a cached catalog, and points the process at
// them via environment variables.
func harness(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "tools.toml"), []byte(`
[tool-index]
expose = "discoverable"
mode = "auto"

[hidden-tool]
expose = "off"
mode = "ask"
`), 0o644)
	cache := filepath.Join(dir, "catalog.json")
	os.WriteFile(cache, []byte(`{
      "built": "2026-09-19T00:00:00Z",
      "tools": [
        {"name":"tool-index","summary":"Discover and describe util-tools",
         "keywords":["registry","discover"]}
      ]
    }`), 0o644)
	t.Setenv("UTIL_TOOLS_HOME", dir)
	t.Setenv("UTIL_TOOLS_CATALOG", cache)
	return dir
}

func TestDescribeSelf(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--describe"}, &out, &errOut); code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	var d struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatalf("descriptor is not JSON: %v", err)
	}
	if d.Name != "tool-index" {
		t.Errorf("name = %q, want tool-index", d.Name)
	}
}

func TestSearchText(t *testing.T) {
	harness(t)
	var out, errOut bytes.Buffer
	if code := run([]string{"search", "registry"}, &out, &errOut); code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "tool-index") {
		t.Errorf("output = %q, want it to mention tool-index", out.String())
	}
}

func TestSearchJSON(t *testing.T) {
	harness(t)
	var out, errOut bytes.Buffer
	if code := run([]string{"--json", "search", "registry"}, &out, &errOut); code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	var got []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v (%q)", err, out.String())
	}
	if len(got) != 1 || got[0].Name != "tool-index" {
		t.Errorf("results = %+v, want one tool-index entry", got)
	}
}

func TestSearchNoMatchExitsZero(t *testing.T) {
	harness(t)
	var out, errOut bytes.Buffer
	if code := run([]string{"search", "kubernetes"}, &out, &errOut); code != 0 {
		t.Errorf("run() = %d, want 0 — no match is not an error", code)
	}
}

func TestDescribeToolNotFound(t *testing.T) {
	harness(t)
	var out, errOut bytes.Buffer
	if code := run([]string{"describe", "absent"}, &out, &errOut); code != 1 {
		t.Errorf("run() = %d, want 1", code)
	}
}

func TestSyncPermissionsDefaultsToDryRun(t *testing.T) {
	harness(t)
	settings := filepath.Join(t.TempDir(), "settings.json")
	original := `{"permissions":{"allow":[]}}`
	os.WriteFile(settings, []byte(original), 0o644)

	var out, errOut bytes.Buffer
	if code := run([]string{"sync-permissions", "--settings", settings}, &out, &errOut); code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Bash(tool-index:*)") {
		t.Errorf("output = %q, want the proposed entry printed", out.String())
	}
	b, _ := os.ReadFile(settings)
	if string(b) != original {
		t.Error("settings file was modified without --write")
	}
}

func TestSyncPermissionsWrite(t *testing.T) {
	harness(t)
	settings := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(settings, []byte(`{"permissions":{"allow":[]}}`), 0o644)

	var out, errOut bytes.Buffer
	if code := run([]string{"sync-permissions", "--settings", settings, "--write"}, &out, &errOut); code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	b, _ := os.ReadFile(settings)
	if !strings.Contains(string(b), "Bash(tool-index:*)") {
		t.Errorf("settings = %s, want the entry added", b)
	}
	// hidden-tool is expose=off and must never be granted.
	if strings.Contains(string(b), "hidden-tool") {
		t.Error("an unexposed tool was granted a permission entry")
	}
}

func TestUnknownSubcommandIsUsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"frobnicate"}, &out, &errOut); code != 2 {
		t.Errorf("run() = %d, want 2", code)
	}
}
