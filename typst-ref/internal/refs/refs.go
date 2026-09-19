// Package refs implements Typst reference-list extraction: pulling Markdown
// reference-link style citations out of a "References" section, filing them
// into a BibTeX-ish bibliography, and rewriting in-text citations to Typst's
// @key syntax. Pure functions: no flags, no printing, no file I/O.
package refs

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Reference is one bibliography entry.
type Reference struct {
	Slug  string
	Title string
	URL   string
}

// Report summarizes what Process did.
type Report struct {
	Added          []Reference
	AlreadyPresent []Reference
	Replaced       int
	Warnings       []string
	SectionFound   bool
	SectionRemoved bool
}

var (
	headingRe   = regexp.MustCompile(`(?m)^(=+)\s*(.+?)\s*$`)
	refDefRe    = regexp.MustCompile(`^\[(\d+)\]:\s*(\S+)\s*"([^"]*)"\s*$`)
	blankLineRe = regexp.MustCompile(`^\s*$`)
)

// Slugify derives a bib entry key from a title: lowercase, non-alphanumeric
// runs collapsed to a single hyphen, leading/trailing hyphens trimmed.
func Slugify(title string) string {
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen && b.Len() > 0 {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// ParseBib parses a references.bib file in the tool's minimal @online format.
func ParseBib(content string) ([]Reference, error) {
	var refs []Reference
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "@online{") {
			continue
		}
		slug := strings.TrimSuffix(strings.TrimPrefix(line, "@online{"), ",")
		slug = strings.TrimSpace(slug)
		var title, url string
		for i++; i < len(lines); i++ {
			l := strings.TrimSpace(lines[i])
			if l == "}" {
				break
			}
			if v, ok := extractBraced(l, "title"); ok {
				title = v
			} else if v, ok := extractBraced(l, "url"); ok {
				url = v
			}
		}
		refs = append(refs, Reference{Slug: slug, Title: title, URL: url})
	}
	return refs, nil
}

func extractBraced(line, field string) (string, bool) {
	prefix := field + " = {"
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(line, prefix)
	rest = strings.TrimSuffix(rest, ",")
	rest = strings.TrimSuffix(rest, "}")
	return rest, true
}

// FormatBib serializes entries sorted by slug, matching the existing
// references.bib style: one blank line between entries, trailing newline.
func FormatBib(refs []Reference) string {
	sorted := make([]Reference, len(refs))
	copy(sorted, refs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Slug < sorted[j].Slug })

	var b strings.Builder
	for i, r := range sorted {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "@online{%s,\n  title = {%s},\n  url = {%s},\n}\n", r.Slug, r.Title, r.URL)
	}
	return b.String()
}

// uniqueSlug returns base, or base with an incrementing numeric suffix, such
// that it is not already present in used.
func uniqueSlug(base string, used map[string]bool) string {
	if !used[base] {
		return base
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if !used[candidate] {
			return candidate
		}
	}
}

// referencesSection locates the References heading (any level) and the span
// of lines it owns: from the line after the heading up to (but not
// including) the next heading of equal or shallower level, or EOF.
type referencesSection struct {
	headingLineIdx int
	level          int
	bodyStart      int // first line index of section body
	bodyEnd        int // exclusive
}

func findReferencesSection(lines []string) *referencesSection {
	for i, line := range lines {
		m := headingRe.FindStringSubmatch(line)
		if m == nil || !strings.EqualFold(m[2], "References") {
			continue
		}
		level := len(m[1])
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			hm := headingRe.FindStringSubmatch(lines[j])
			if hm != nil && len(hm[1]) <= level {
				end = j
				break
			}
		}
		return &referencesSection{headingLineIdx: i, level: level, bodyStart: i + 1, bodyEnd: end}
	}
	return nil
}

// Process extracts references from typContent's References section, merges
// them into bibContent (which may be empty), rewrites in-text citations to
// @slug form, and removes the recognized reference definitions (and the
// section itself, if nothing unrecognized remains in it).
func Process(typContent, bibContent string) (newTyp string, newBib string, report Report, err error) {
	existing, err := ParseBib(bibContent)
	if err != nil {
		return "", "", Report{}, err
	}

	lines := strings.Split(typContent, "\n")
	section := findReferencesSection(lines)
	if section == nil {
		return typContent, "", Report{}, nil
	}
	report.SectionFound = true

	usedSlugs := make(map[string]bool, len(existing))
	urlToSlug := make(map[string]string, len(existing))
	for _, r := range existing {
		usedSlugs[r.Slug] = true
		urlToSlug[r.URL] = r.Slug
	}

	type parsedDef struct {
		num  string
		ref  Reference
		line int
	}
	var defs []parsedDef
	keepLines := make(map[int]bool) // body lines that stay (unrecognized, non-blank)

	for i := section.bodyStart; i < section.bodyEnd; i++ {
		raw := lines[i]
		trimmed := strings.TrimRight(raw, "\r")
		if blankLineRe.MatchString(trimmed) {
			continue
		}
		m := refDefRe.FindStringSubmatch(strings.TrimSpace(trimmed))
		if m == nil {
			keepLines[i] = true
			report.Warnings = append(report.Warnings, fmt.Sprintf("references section: unrecognized line %d left untouched: %q", i+1, trimmed))
			continue
		}
		num, url, title := m[1], m[2], m[3]
		var slug string
		if existingSlug, ok := urlToSlug[url]; ok {
			slug = existingSlug
			report.AlreadyPresent = append(report.AlreadyPresent, Reference{Slug: slug, Title: title, URL: url})
		} else {
			slug = uniqueSlug(Slugify(title), usedSlugs)
			usedSlugs[slug] = true
			urlToSlug[url] = slug
			ref := Reference{Slug: slug, Title: title, URL: url}
			existing = append(existing, ref)
			report.Added = append(report.Added, ref)
		}
		defs = append(defs, parsedDef{num: num, ref: Reference{Slug: slug, Title: title, URL: url}, line: i})
	}

	// Rewrite in-text citations: ([Text][N]) -> @slug, falling back to
	// [Text][N] -> @slug when not paren-wrapped.
	textBefore := strings.Join(lines[:section.headingLineIdx], "\n")
	for _, d := range defs {
		num := regexp.QuoteMeta(d.num)
		parenRe := regexp.MustCompile(`\(\s*\[[^\]\n]*\]\[` + num + `\]\s*\)`)
		bareRe := regexp.MustCompile(`\[[^\]\n]*\]\[` + num + `\]`)

		if n := len(parenRe.FindAllStringIndex(textBefore, -1)); n > 0 {
			report.Replaced += n
			textBefore = parenRe.ReplaceAllString(textBefore, "@"+d.ref.Slug)
		}
		if n := len(bareRe.FindAllStringIndex(textBefore, -1)); n > 0 {
			report.Replaced += n
			textBefore = bareRe.ReplaceAllString(textBefore, "@"+d.ref.Slug)
		}
	}
	body := textBefore

	// Rebuild the section: drop recognized definition lines and blank lines
	// between them; keep unrecognized lines as-is.
	var sectionBody []string
	for i := section.bodyStart; i < section.bodyEnd; i++ {
		if keepLines[i] {
			sectionBody = append(sectionBody, lines[i])
		}
	}
	sectionHasContent := false
	for _, l := range sectionBody {
		if !blankLineRe.MatchString(l) {
			sectionHasContent = true
			break
		}
	}

	var out strings.Builder
	out.WriteString(body)
	if sectionHasContent {
		if body != "" && !strings.HasSuffix(body, "\n") {
			out.WriteString("\n")
		}
		out.WriteString(lines[section.headingLineIdx])
		out.WriteString("\n")
		out.WriteString(strings.Join(sectionBody, "\n"))
		if section.bodyEnd < len(lines) {
			out.WriteString("\n")
			out.WriteString(strings.Join(lines[section.bodyEnd:], "\n"))
		}
	} else {
		report.SectionRemoved = true
		if section.bodyEnd < len(lines) {
			rest := strings.Join(lines[section.bodyEnd:], "\n")
			if body != "" && !strings.HasSuffix(body, "\n") {
				out.WriteString("\n")
			}
			out.WriteString(rest)
		}
	}

	newTyp = out.String()
	if len(report.Added) > 0 {
		newBib = FormatBib(existing)
	} else if bibContent != "" {
		newBib = bibContent
	} else {
		newBib = ""
	}
	return newTyp, newBib, report, nil
}
