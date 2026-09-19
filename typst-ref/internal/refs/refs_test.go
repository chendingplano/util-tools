package refs

import (
	"os"
	"strings"
	"testing"
)

const docExample = `Jev evaluates every question independently against the same supplied state. Users can
supply multiple questions with the same context. Jev answers them all in one call ([TypeSafe AI][1]).

Jev interprets the supplied information using knowledge encoded in its model parameters, much as an LLM
understands the semantics of text. But Jev's job is *not knowledge retrieval*. Its job is to make a fast
judgment over the state you supplied. TypeSafe explicitly characterizes good Jev questions as judgments
that a knowledgeable person could make quickly *"given the right context."* ([TypeSafe AI][1])

That fits remarkably well with the *candidate to investigative exploration* architecture we
discussed for SemOS: conventional retrieval finds candidates; Jev can cheaply perform many of
the fuzzy judgments that determine *which candidates to keep and where to
explore next*. ([TypeSafe AI][2])

= References
[1]: https://docs.typesafe.ai/introduction "Introduction - TypeSafe AI"
[2]: https://typesafe.ai/blog/introducing-system-one-models-and-jev?utm_source=chatgpt.com "Introducing System One Models and Jev - TypeSafe AI Blog"
`

func TestSlugify(t *testing.T) {
	tests := []struct{ title, want string }{
		{"Introduction - TypeSafe AI", "introduction-typesafe-ai"},
		{"Introducing System One Models and Jev - TypeSafe AI Blog", "introducing-system-one-models-and-jev-typesafe-ai-blog"},
		{"  Leading/trailing punctuation!!  ", "leading-trailing-punctuation"},
		{"Multiple   spaces --- and -- dashes", "multiple-spaces-and-dashes"},
	}
	for _, tt := range tests {
		if got := Slugify(tt.title); got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}

func TestParseAndFormatBib(t *testing.T) {
	src := `@online{context-engineering,
  title = {Understanding Context Engineering},
  url = {https://dzone.com/articles/understanding-context-engineering},
}

@online{gstack,
  title = {gstack},
  url = {https://github.com/garrytan/gstack},
}
`
	got, err := ParseBib(src)
	if err != nil {
		t.Fatalf("ParseBib() error = %v", err)
	}
	want := []Reference{
		{Slug: "context-engineering", Title: "Understanding Context Engineering", URL: "https://dzone.com/articles/understanding-context-engineering"},
		{Slug: "gstack", Title: "gstack", URL: "https://github.com/garrytan/gstack"},
	}
	if len(got) != len(want) {
		t.Fatalf("ParseBib() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	formatted := FormatBib(got)
	roundTrip, err := ParseBib(formatted)
	if err != nil {
		t.Fatalf("ParseBib(FormatBib()) error = %v", err)
	}
	if len(roundTrip) != len(want) {
		t.Fatalf("round trip = %+v, want %+v", roundTrip, want)
	}
}

func TestFormatBibSortedBySlug(t *testing.T) {
	refs := []Reference{
		{Slug: "zebra", Title: "Zebra", URL: "https://example.com/z"},
		{Slug: "alpha", Title: "Alpha", URL: "https://example.com/a"},
	}
	out := FormatBib(refs)
	if strings.Index(out, "alpha") > strings.Index(out, "zebra") {
		t.Errorf("FormatBib() not sorted by slug:\n%s", out)
	}
}

func TestProcessDocExample(t *testing.T) {
	newTyp, newBib, report, err := Process(docExample, "")
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if len(report.Added) != 2 {
		t.Fatalf("report.Added = %+v, want 2 entries", report.Added)
	}
	if report.Added[0].Slug != "introduction-typesafe-ai" {
		t.Errorf("Added[0].Slug = %q, want %q", report.Added[0].Slug, "introduction-typesafe-ai")
	}
	if report.Added[1].Slug != "introducing-system-one-models-and-jev-typesafe-ai-blog" {
		t.Errorf("Added[1].Slug = %q, want %q", report.Added[1].Slug, "introducing-system-one-models-and-jev-typesafe-ai-blog")
	}
	if report.Replaced != 3 {
		t.Errorf("report.Replaced = %d, want 3", report.Replaced)
	}
	if !report.SectionRemoved {
		t.Errorf("report.SectionRemoved = false, want true")
	}

	if strings.Contains(newTyp, "= References") {
		t.Errorf("newTyp still contains References section:\n%s", newTyp)
	}
	if strings.Contains(newTyp, "[1]") || strings.Contains(newTyp, "[2]") {
		t.Errorf("newTyp still contains bracket citations:\n%s", newTyp)
	}
	if !strings.Contains(newTyp, "@introduction-typesafe-ai") {
		t.Errorf("newTyp missing @introduction-typesafe-ai:\n%s", newTyp)
	}
	if !strings.Contains(newTyp, "@introducing-system-one-models-and-jev-typesafe-ai-blog") {
		t.Errorf("newTyp missing @introducing-system-one-models-and-jev-typesafe-ai-blog:\n%s", newTyp)
	}
	// Parens wrapping a single citation are removed along with it.
	if strings.Contains(newTyp, "(@introduction-typesafe-ai)") {
		t.Errorf("newTyp did not strip wrapping parens:\n%s", newTyp)
	}

	if !strings.Contains(newBib, "@online{introduction-typesafe-ai,") {
		t.Errorf("newBib missing introduction-typesafe-ai entry:\n%s", newBib)
	}
	if !strings.Contains(newBib, "@online{introducing-system-one-models-and-jev-typesafe-ai-blog,") {
		t.Errorf("newBib missing blog entry:\n%s", newBib)
	}
}

func TestProcessSkipsExistingURL(t *testing.T) {
	typContent := `See ([TypeSafe AI][1]).

= References
[1]: https://docs.typesafe.ai/introduction "Introduction - TypeSafe AI"
`
	bibContent := `@online{introduction-typesafe-ai,
  title = {Introduction - TypeSafe AI},
  url = {https://docs.typesafe.ai/introduction},
}
`
	_, newBib, report, err := Process(typContent, bibContent)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(report.Added) != 0 {
		t.Errorf("report.Added = %+v, want none (already present)", report.Added)
	}
	if len(report.AlreadyPresent) != 1 || report.AlreadyPresent[0].Slug != "introduction-typesafe-ai" {
		t.Errorf("report.AlreadyPresent = %+v", report.AlreadyPresent)
	}
	// bib content unchanged (still exactly one entry)
	count := strings.Count(newBib, "@online{")
	if count != 1 {
		t.Errorf("newBib has %d entries, want 1:\n%s", count, newBib)
	}
}

func TestProcessSlugCollisionDifferentURL(t *testing.T) {
	typContent := `([Foo][1])

= References
[1]: https://example.com/foo "Foo"
`
	bibContent := `@online{foo,
  title = {Foo},
  url = {https://example.com/other-foo},
}
`
	_, newBib, report, err := Process(typContent, bibContent)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(report.Added) != 1 {
		t.Fatalf("report.Added = %+v, want 1", report.Added)
	}
	if report.Added[0].Slug == "foo" {
		t.Errorf("expected slug collision to be resolved, got reused slug %q", report.Added[0].Slug)
	}
	if !strings.Contains(newBib, report.Added[0].Slug) {
		t.Errorf("newBib missing resolved slug %q:\n%s", report.Added[0].Slug, newBib)
	}
}

func TestProcessUnrecognizedLineLeftInPlace(t *testing.T) {
	typContent := `([Foo][1])

= References
[1]: https://example.com/foo "Foo"
[2] Some Title, https://example.com/bar
`
	_, _, report, err := Process(typContent, "")
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(report.Warnings) == 0 {
		t.Fatalf("expected a warning for the unrecognized reference line")
	}
	if report.SectionRemoved {
		t.Errorf("report.SectionRemoved = true, want false (unrecognized line remains)")
	}
	newTyp, _, _, _ := Process(typContent, "")
	if !strings.Contains(newTyp, "[2] Some Title, https://example.com/bar") {
		t.Errorf("unrecognized line was altered:\n%s", newTyp)
	}
	if !strings.Contains(newTyp, "= References") {
		t.Errorf("References heading removed even though unrecognized content remains:\n%s", newTyp)
	}
}

func TestProcessNoReferencesSection(t *testing.T) {
	typContent := "Just some text with no references at all.\n"
	newTyp, newBib, report, err := Process(typContent, "")
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if newTyp != typContent {
		t.Errorf("newTyp = %q, want unchanged %q", newTyp, typContent)
	}
	if newBib != "" {
		t.Errorf("newBib = %q, want empty", newBib)
	}
	if len(report.Added) != 0 || len(report.Warnings) != 0 {
		t.Errorf("report = %+v, want empty", report)
	}
}

func TestProcessGoldenFixture(t *testing.T) {
	src, err := os.ReadFile("../../testdata/sample.typ")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/sample.want.typ")
	if err != nil {
		t.Fatal(err)
	}
	newTyp, _, _, err := Process(string(src), "")
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if newTyp != string(want) {
		t.Errorf("Process(testdata/sample.typ) = %q, want %q", newTyp, string(want))
	}
}

func TestEnsureBibliographyAddsWhenMissing(t *testing.T) {
	content := "Body text @introduction-typesafe-ai.\n"
	got, added := EnsureBibliography(content, "../references/references.bib")
	if !added {
		t.Fatalf("EnsureBibliography() added = false, want true")
	}
	if !strings.Contains(got, `#bibliography("../references/references.bib")`) {
		t.Errorf("EnsureBibliography() = %q, missing #bibliography call", got)
	}
	if !strings.HasPrefix(got, content) {
		t.Errorf("EnsureBibliography() should append, got %q", got)
	}
}

func TestEnsureBibliographyNoopWhenPresent(t *testing.T) {
	content := "Body text @x.\n\n#bibliography(\"references/references.bib\")\n"
	got, added := EnsureBibliography(content, "references/references.bib")
	if added {
		t.Errorf("EnsureBibliography() added = true, want false (already present)")
	}
	if got != content {
		t.Errorf("EnsureBibliography() modified content when a call already exists:\n%s", got)
	}
}

func TestEnsureBibliographyNoopWhenPathEmpty(t *testing.T) {
	content := "Body text, no citations.\n"
	got, added := EnsureBibliography(content, "")
	if added || got != content {
		t.Errorf("EnsureBibliography() with empty path should be a no-op")
	}
}

func TestProcessAddsBibliographyCallViaEnsure(t *testing.T) {
	newTyp, _, report, err := Process(docExample, "")
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if report.Replaced == 0 {
		t.Fatalf("expected citations to be replaced")
	}
	final, added := EnsureBibliography(newTyp, "../references/references.bib")
	if !added {
		t.Fatalf("expected EnsureBibliography to add a call after Process")
	}
	if !strings.Contains(final, `#bibliography("../references/references.bib")`) {
		t.Errorf("final content missing bibliography call:\n%s", final)
	}
}

func TestProcessSubsectionHeading(t *testing.T) {
	typContent := `Body text ([X][1]).

== References
[1]: https://example.com/x "X"

== Next Section
More text.
`
	newTyp, _, report, err := Process(typContent, "")
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(report.Added) != 1 {
		t.Fatalf("report.Added = %+v, want 1", report.Added)
	}
	if !strings.Contains(newTyp, "== Next Section") {
		t.Errorf("newTyp lost the following section:\n%s", newTyp)
	}
	if strings.Contains(newTyp, "== References") {
		t.Errorf("newTyp still has References heading:\n%s", newTyp)
	}
}
