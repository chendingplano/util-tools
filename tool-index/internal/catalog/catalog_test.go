package catalog

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

func fakeRunner(descriptors map[string]string) Runner {
	return func(name string) ([]byte, error) {
		body, ok := descriptors[name]
		if !ok {
			return nil, fmt.Errorf("not installed: %s", name)
		}
		return []byte(body), nil
	}
}

var fixtures = map[string]string{
	"typst-ref": `{"name":"typst-ref","summary":"Extract Typst references into a .bib",
	               "keywords":["typst","bibtex","references","citations"]}`,
	"tool-index": `{"name":"tool-index","summary":"Discover and describe util-tools",
	                "keywords":["registry","discover","tools"]}`,
}

func TestBuildCollectsDescriptors(t *testing.T) {
	c, errs := Build([]string{"typst-ref", "tool-index"}, fakeRunner(fixtures))
	if len(errs) != 0 {
		t.Fatalf("Build() errors = %v", errs)
	}
	if len(c.Tools) != 2 {
		t.Fatalf("Tools = %d, want 2", len(c.Tools))
	}
	if c.Built.IsZero() {
		t.Error("Built timestamp not set")
	}
}

func TestBuildReportsMissingWithoutFailing(t *testing.T) {
	// A tool in policy but not installed is reported, not fatal: the rest of
	// the catalog must still build.
	c, errs := Build([]string{"typst-ref", "never-built"}, fakeRunner(fixtures))
	if len(errs) != 1 {
		t.Fatalf("errors = %v, want exactly 1", errs)
	}
	if len(c.Tools) != 1 {
		t.Errorf("Tools = %d, want 1 surviving entry", len(c.Tools))
	}
}

func TestSearch(t *testing.T) {
	c, _ := Build([]string{"typst-ref", "tool-index"}, fakeRunner(fixtures))
	tests := []struct {
		name  string
		query []string
		want  []string
	}{
		{"keyword match", []string{"bibtex"}, []string{"typst-ref"}},
		{"name match", []string{"typst"}, []string{"typst-ref"}},
		{"summary match", []string{"discover"}, []string{"tool-index"}},
		{"case insensitive", []string{"BibTeX"}, []string{"typst-ref"}},
		{"no match", []string{"kubernetes"}, nil},
		{"empty query returns all", nil, []string{"tool-index", "typst-ref"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, d := range c.Search(tt.query) {
				got = append(got, d.Name)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Search(%v) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestSearchRanksMoreMatchesFirst(t *testing.T) {
	c, _ := Build([]string{"typst-ref", "tool-index"}, fakeRunner(fixtures))
	// typst-ref matches "typst" and "references" (2); tool-index matches
	// "discover" (1). Ranking by match count must beat the alphabetical
	// tie-break, which would otherwise put tool-index first.
	got := c.Search([]string{"typst", "references", "discover"})
	if len(got) != 2 {
		t.Fatalf("Search() = %d results, want 2", len(got))
	}
	if got[0].Name != "typst-ref" {
		t.Errorf("first result = %q, want typst-ref (2 term matches beats 1)", got[0].Name)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	c, _ := Build([]string{"typst-ref"}, fakeRunner(fixtures))
	path := filepath.Join(t.TempDir(), "nested", "catalog.json")
	if err := Save(path, c); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Tools) != 1 || got.Tools[0].Name != "typst-ref" {
		t.Errorf("round trip = %+v, want one typst-ref entry", got.Tools)
	}
}

func TestGet(t *testing.T) {
	c, _ := Build([]string{"typst-ref"}, fakeRunner(fixtures))
	if _, ok := c.Get("typst-ref"); !ok {
		t.Error("Get(typst-ref) = not found, want found")
	}
	if _, ok := c.Get("absent"); ok {
		t.Error("Get(absent) = found, want not found")
	}
}
