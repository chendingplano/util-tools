package permissions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const settings = `{
  "permissions": {
    "allow": ["Bash(ls:*)", "Bash(tool-index:*)"],
    "deny": ["Bash(rm:*)"]
  },
  "env": {"FOO": "bar"}
}`

func TestEntry(t *testing.T) {
	if got, want := Entry("typst-ref"), "Bash(typst-ref:*)"; got != want {
		t.Errorf("Entry() = %q, want %q", got, want)
	}
}

func TestMissing(t *testing.T) {
	existing := []string{"Bash(ls:*)", "Bash(tool-index:*)"}
	got := Missing([]string{"tool-index", "typst-ref"}, existing)
	want := []string{"Bash(typst-ref:*)"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Missing() = %v, want %v", got, want)
	}
}

func TestReadAllow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte(settings), 0o644)
	got, err := ReadAllow(path)
	if err != nil {
		t.Fatalf("ReadAllow() error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("ReadAllow() = %v, want 2 entries", got)
	}
}

func TestApplyPreservesOtherKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, []byte(settings), 0o644)

	if err := Apply(path, []string{"Bash(typst-ref:*)"}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	var got struct {
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
		} `json:"permissions"`
		Env map[string]string `json:"env"`
	}
	b, _ := os.ReadFile(path)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if len(got.Permissions.Allow) != 3 {
		t.Errorf("allow = %v, want 3 entries", got.Permissions.Allow)
	}
	if len(got.Permissions.Deny) != 1 {
		t.Errorf("deny lost: %v", got.Permissions.Deny)
	}
	if got.Env["FOO"] != "bar" {
		t.Errorf("env lost: %v", got.Env)
	}
}

func TestApplyWritesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, []byte(settings), 0o644)

	if err := Apply(path, []string{"Bash(typst-ref:*)"}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	b, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("backup not written: %v", err)
	}
	if string(b) != settings {
		t.Error("backup does not match the original file")
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, []byte(settings), 0o644)

	Apply(path, []string{"Bash(typst-ref:*)"})
	Apply(path, []string{"Bash(typst-ref:*)"})

	allow, _ := ReadAllow(path)
	count := 0
	for _, a := range allow {
		if a == "Bash(typst-ref:*)" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("entry appears %d times, want 1", count)
	}
}
