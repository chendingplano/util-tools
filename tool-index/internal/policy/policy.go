// Package policy reads the agent-exposure policy for util-tools.
//
// Descriptive metadata about a tool comes from the tool binary itself
// (see package describe). This package answers only the policy question:
// may an agent see and run it.
package policy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

// Expose controls whether agents can discover a tool.
type Expose string

// Mode controls whether agents are prompted before running a tool.
type Mode string

const (
	ExposeOff          Expose = "off"
	ExposeDiscoverable Expose = "discoverable"

	ModeAsk  Mode = "ask"
	ModeAuto Mode = "auto"
)

// Entry is one tool's policy. The zero value is the safe default:
// invisible to agents, and prompted if run.
type Entry struct {
	Expose Expose `toml:"expose"`
	Mode   Mode   `toml:"mode"`
}

// Set maps tool name to policy.
type Set map[string]Entry

const (
	fileName      = "tools.toml"
	localFileName = "tools.local.toml"
)

// FindDir locates the directory holding tools.toml, checking
// $UTIL_TOOLS_CONFIG (a file path), then $UTIL_TOOLS_HOME, then the user
// config directory.
func FindDir() (string, error) {
	if p := os.Getenv("UTIL_TOOLS_CONFIG"); p != "" {
		return filepath.Dir(p), nil
	}
	if p := os.Getenv("UTIL_TOOLS_HOME"); p != "" {
		return p, nil
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("policy: cannot locate config dir: %w", err)
	}
	return filepath.Join(cfg, "util-tools"), nil
}

// Load reads tools.toml from dir and overlays tools.local.toml if present.
// A missing tools.toml is an error; a missing tools.local.toml is not.
func Load(dir string) (Set, error) {
	base, err := readFile(filepath.Join(dir, fileName))
	if err != nil {
		return nil, err
	}
	local, err := readFile(filepath.Join(dir, localFileName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for name, e := range local {
		base[name] = e
	}
	for name, e := range base {
		if err := e.validate(name); err != nil {
			return nil, err
		}
	}
	return base, nil
}

func readFile(path string) (Set, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("policy: reading %s: %w", path, err)
	}
	var s Set
	if err := toml.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("policy: parsing %s: %w", path, err)
	}
	for name, e := range s {
		if e.Expose == "" {
			e.Expose = ExposeOff
		}
		if e.Mode == "" {
			e.Mode = ModeAsk
		}
		s[name] = e
	}
	return s, nil
}

func (e Entry) validate(name string) error {
	switch e.Expose {
	case ExposeOff, ExposeDiscoverable:
	default:
		return fmt.Errorf("policy: tool %q: unknown expose %q (want %q or %q)",
			name, e.Expose, ExposeOff, ExposeDiscoverable)
	}
	switch e.Mode {
	case ModeAsk, ModeAuto:
	default:
		return fmt.Errorf("policy: tool %q: unknown mode %q (want %q or %q)",
			name, e.Mode, ModeAsk, ModeAuto)
	}
	return nil
}

// Discoverable returns the sorted names of tools agents may discover.
func (s Set) Discoverable() []string {
	return s.filter(func(e Entry) bool { return e.Expose == ExposeDiscoverable })
}

// AutoMode returns the sorted names of discoverable tools that run without
// prompting. An unexposed tool is never returned, whatever its mode.
func (s Set) AutoMode() []string {
	return s.filter(func(e Entry) bool {
		return e.Expose == ExposeDiscoverable && e.Mode == ModeAuto
	})
}

func (s Set) filter(keep func(Entry) bool) []string {
	var out []string
	for name, e := range s {
		if keep(e) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
