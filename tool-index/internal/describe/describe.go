// Package describe defines the tool descriptor that every util-tools tool
// emits from its --describe flag, and that tool-index consumes.
package describe

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Arg documents one positional argument or flag of a tool.
type Arg struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
	Default  string `json:"default,omitempty"`
	Help     string `json:"help,omitempty"`
}

// Descriptor is a tool's self-description.
type Descriptor struct {
	Name     string   `json:"name"`
	Summary  string   `json:"summary"`
	Keywords []string `json:"keywords"`
	Args     []Arg    `json:"args,omitempty"`
	Examples []string `json:"examples,omitempty"`
}

// Validate reports whether d meets the contract in AGENTS.md.
func (d Descriptor) Validate() error {
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("descriptor: name is required")
	}
	if strings.TrimSpace(d.Summary) == "" {
		return fmt.Errorf("descriptor %q: summary is required", d.Name)
	}
	if strings.ContainsAny(d.Summary, "\r\n") {
		return fmt.Errorf("descriptor %q: summary must be a single line", d.Name)
	}
	if len(d.Keywords) == 0 {
		return fmt.Errorf("descriptor %q: at least one keyword is required", d.Name)
	}
	return nil
}

// Parse decodes and validates a descriptor emitted by a tool's --describe.
func Parse(b []byte) (Descriptor, error) {
	var d Descriptor
	if err := json.Unmarshal(b, &d); err != nil {
		return Descriptor{}, fmt.Errorf("descriptor: invalid JSON: %w", err)
	}
	if err := d.Validate(); err != nil {
		return Descriptor{}, err
	}
	return d, nil
}
