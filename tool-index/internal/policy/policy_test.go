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
[widget-tool]
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
	if got, want := s.Discoverable(), []string{"widget-tool"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Discoverable() = %v, want %v", got, want)
	}
	if got, want := s.AutoMode(), []string{"widget-tool"}; !reflect.DeepEqual(got, want) {
		t.Errorf("AutoMode() = %v, want %v", got, want)
	}
}

func TestLoadRejectsSelfAuto(t *testing.T) {
	// tool-index is the permission-granting tool itself: mode=auto on it
	// would let an agent, once allowed to run tool-index unprompted, use
	// sync-permissions --write to grant itself arbitrary further
	// permissions. Load must reject this regardless of expose.
	dir := t.TempDir()
	write(t, dir, "tools.toml", "[tool-index]\nexpose = \"discoverable\"\nmode = \"auto\"\n")
	if _, err := Load(dir); err == nil {
		t.Error("Load() = nil error, want error for tool-index mode=auto")
	}
}

func TestLoadAllowsOtherToolAuto(t *testing.T) {
	// The restriction is specific to tool-index; any other tool may use
	// mode=auto freely.
	dir := t.TempDir()
	write(t, dir, "tools.toml", "[typst-ref]\nexpose = \"discoverable\"\nmode = \"auto\"\n")
	if _, err := Load(dir); err != nil {
		t.Errorf("Load() error = %v, want nil for a non-self tool at mode=auto", err)
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
