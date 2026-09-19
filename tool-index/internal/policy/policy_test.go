package policy

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "tools.toml", `
[tool-index]
expose = "discoverable"
mode = "auto"

[quiet-tool]
expose = "off"
mode = "ask"

[sparse-tool]
`)
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// An entry with no fields defaults to the safe values.
	if got := s["sparse-tool"]; got.Expose != ExposeOff || got.Mode != ModeAsk {
		t.Errorf("sparse-tool = %+v, want off/ask", got)
	}
	if got, want := s.Discoverable(), []string{"tool-index"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Discoverable() = %v, want %v", got, want)
	}
	if got, want := s.AutoMode(), []string{"tool-index"}; !reflect.DeepEqual(got, want) {
		t.Errorf("AutoMode() = %v, want %v", got, want)
	}
}

func TestLocalOverlayWins(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "tools.toml", "[a]\nexpose = \"off\"\nmode = \"ask\"\n[b]\nexpose = \"discoverable\"\nmode = \"ask\"\n")
	write(t, dir, "tools.local.toml", "[a]\nexpose = \"discoverable\"\nmode = \"auto\"\n")
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := s["a"]; got.Expose != ExposeDiscoverable || got.Mode != ModeAuto {
		t.Errorf("a = %+v, want discoverable/auto", got)
	}
	if got := s["b"]; got.Expose != ExposeDiscoverable || got.Mode != ModeAsk {
		t.Errorf("b = %+v, want discoverable/ask (untouched by overlay)", got)
	}
}

func TestLoadRejectsUnknownValues(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "tools.toml", "[a]\nexpose = \"maybe\"\n")
	if _, err := Load(dir); err == nil {
		t.Error("Load() = nil error, want error for unknown expose value")
	}
}

func TestAutoModeExcludesUnexposed(t *testing.T) {
	dir := t.TempDir()
	// mode=auto on an unexposed tool must never produce a permission grant.
	write(t, dir, "tools.toml", "[a]\nexpose = \"off\"\nmode = \"auto\"\n")
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := s.AutoMode(); len(got) != 0 {
		t.Errorf("AutoMode() = %v, want empty for an unexposed tool", got)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Error("Load() = nil error, want error when tools.toml is absent")
	}
}
