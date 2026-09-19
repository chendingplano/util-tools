// Package permissions turns "auto" mode policy into Claude Code allowlist
// entries.
package permissions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Entry is the allowlist string granting unprompted use of a tool.
func Entry(tool string) string {
	return fmt.Sprintf("Bash(%s:*)", tool)
}

// Missing returns the sorted allowlist entries for tools that are not already
// present in existing.
func Missing(tools []string, existing []string) []string {
	have := make(map[string]bool, len(existing))
	for _, e := range existing {
		have[e] = true
	}
	var out []string
	for _, t := range tools {
		if e := Entry(t); !have[e] {
			out = append(out, e)
		}
	}
	sort.Strings(out)
	return out
}

// ReadAllow returns the permissions.allow list from a settings file.
func ReadAllow(path string) ([]string, error) {
	_, perms, err := read(path)
	if err != nil {
		return nil, err
	}
	return perms.Allow, nil
}

type permsBlock struct {
	Allow []string                   `json:"allow"`
	Rest  map[string]json.RawMessage `json:"-"`
}

func read(path string) (map[string]json.RawMessage, permsBlock, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, permsBlock{}, fmt.Errorf("permissions: %w", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		return nil, permsBlock{}, fmt.Errorf("permissions: parsing %s: %w", path, err)
	}
	var perms permsBlock
	perms.Rest = map[string]json.RawMessage{}
	if raw, ok := top["permissions"]; ok {
		if err := json.Unmarshal(raw, &perms.Rest); err != nil {
			return nil, permsBlock{}, fmt.Errorf("permissions: parsing permissions block: %w", err)
		}
		if raw, ok := perms.Rest["allow"]; ok {
			if err := json.Unmarshal(raw, &perms.Allow); err != nil {
				return nil, permsBlock{}, fmt.Errorf("permissions: parsing allow list: %w", err)
			}
		}
	}
	return top, perms, nil
}

// Apply adds the given allowlist entries to the settings file, skipping any
// already present. The original is copied to <path>.bak first, and the new
// file is written atomically. Keys other than permissions.allow are carried
// through untouched as raw JSON.
func Apply(path string, add []string) error {
	top, perms, err := read(path)
	if err != nil {
		return err
	}

	have := make(map[string]bool, len(perms.Allow))
	for _, a := range perms.Allow {
		have[a] = true
	}
	changed := false
	for _, a := range add {
		if !have[a] {
			perms.Allow = append(perms.Allow, a)
			have[a] = true
			changed = true
		}
	}
	if !changed {
		return nil
	}

	orig, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("permissions: %w", err)
	}
	if err := os.WriteFile(path+".bak", orig, 0o644); err != nil {
		return fmt.Errorf("permissions: writing backup: %w", err)
	}

	allowRaw, err := json.Marshal(perms.Allow)
	if err != nil {
		return fmt.Errorf("permissions: %w", err)
	}
	perms.Rest["allow"] = allowRaw
	permsRaw, err := json.Marshal(perms.Rest)
	if err != nil {
		return fmt.Errorf("permissions: %w", err)
	}
	top["permissions"] = permsRaw

	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return fmt.Errorf("permissions: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "settings-*.json")
	if err != nil {
		return fmt.Errorf("permissions: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(out, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("permissions: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("permissions: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("permissions: %w", err)
	}
	return nil
}
