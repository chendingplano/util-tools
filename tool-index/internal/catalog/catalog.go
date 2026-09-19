// Package catalog builds and queries the registry of installed util-tools.
//
// It is a registry, not a dispatcher: it can find and describe tools, and
// deliberately cannot run them.
package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chendingplano/util-tools/tool-index/internal/describe"
)

// Runner returns the raw --describe output of the named tool.
type Runner func(name string) ([]byte, error)

// Catalog is the cached set of descriptors for installed, exposed tools.
type Catalog struct {
	Built time.Time             `json:"built"`
	Tools []describe.Descriptor `json:"tools"`
}

// ExecRunner runs `<name> --describe` by resolving name on PATH.
func ExecRunner(name string) ([]byte, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("%s: not installed: %w", name, err)
	}
	out, err := exec.Command(path, "--describe").Output()
	if err != nil {
		return nil, fmt.Errorf("%s --describe: %w", name, err)
	}
	return out, nil
}

// Build collects descriptors for names. A tool that is missing or emits an
// invalid descriptor is reported in the error slice but does not prevent the
// rest of the catalog from building.
func Build(names []string, run Runner) (Catalog, []error) {
	c := Catalog{Built: time.Now().UTC()}
	var errs []error
	for _, name := range names {
		out, err := run(name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		d, err := describe.Parse(out)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if d.Name != name {
			errs = append(errs, fmt.Errorf("%s: descriptor name is %q; they must match", name, d.Name))
			continue
		}
		c.Tools = append(c.Tools, d)
	}
	sort.Slice(c.Tools, func(i, j int) bool { return c.Tools[i].Name < c.Tools[j].Name })
	return c, errs
}

// Get returns the descriptor for an exact tool name.
func (c Catalog) Get(name string) (describe.Descriptor, bool) {
	for _, d := range c.Tools {
		if d.Name == name {
			return d, true
		}
	}
	return describe.Descriptor{}, false
}

// Search returns descriptors matching any query term, most matches first,
// ties broken by name. An empty query returns everything.
func (c Catalog) Search(query []string) []describe.Descriptor {
	if len(query) == 0 {
		out := make([]describe.Descriptor, len(c.Tools))
		copy(out, c.Tools)
		return out
	}

	type scored struct {
		d     describe.Descriptor
		score int
	}
	var hits []scored
	for _, d := range c.Tools {
		haystack := strings.ToLower(strings.Join(
			append([]string{d.Name, d.Summary}, d.Keywords...), " "))
		score := 0
		for _, term := range query {
			if strings.Contains(haystack, strings.ToLower(term)) {
				score++
			}
		}
		if score > 0 {
			hits = append(hits, scored{d, score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].d.Name < hits[j].d.Name
	})

	// Initialised (not nil) so an empty result marshals as JSON "[]" rather
	// than "null" — a JSON consumer iterating or calling .length on a search
	// result must not have to special-case no matches.
	out := []describe.Descriptor{}
	for _, h := range hits {
		out = append(out, h.d)
	}
	return out
}

// DefaultPath is where the cached catalog lives.
func DefaultPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("catalog: cannot locate cache dir: %w", err)
	}
	return filepath.Join(dir, "util-tools", "catalog.json"), nil
}

// Save writes the catalog atomically, creating parent directories as needed.
func Save(path string, c Catalog) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("catalog: %w", err)
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("catalog: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "catalog-*.json")
	if err != nil {
		return fmt.Errorf("catalog: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("catalog: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("catalog: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("catalog: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("catalog: %w", err)
	}
	return nil
}

// Load reads a cached catalog.
func Load(path string) (Catalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("catalog: %w", err)
	}
	var c Catalog
	if err := json.Unmarshal(b, &c); err != nil {
		return Catalog{}, fmt.Errorf("catalog: parsing %s: %w", path, err)
	}
	return c, nil
}
