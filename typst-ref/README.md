# typst-ref

Extracts a Typst file's `References` section into `references.bib`, rewrites
in-text citations to Typst's `@key` syntax, and removes the section once
everything in it has been filed.

## What it recognizes

Only the Markdown reference-link definition form:

```text
[1]: https://example.com/page "Page Title"
```

used in text as `[Some Text][1]` (optionally wrapped in parens, which are
removed along with the citation: `([Some Text][1])` -> `@page-title`).

Any other line inside the `References` section (numbered prose refs,
`[[file_id:...]]` links, etc.) is left untouched and reported as a warning —
recognizing free-form prose references is out of scope and error-prone. If a
`References` section still contains such lines after processing, the section
heading and those lines are kept; the section is removed only once nothing
unrecognized remains in it.

## Bibliography

Entries are `@online{slug, title = {...}, url = {...}}`, sorted by slug,
deduplicated by `url`. A slug collision with a different `url` gets a `-2`,
`-3`, ... suffix.

## Locating references.bib

By default, `typst-ref` walks up from the input file's directory looking for
a `references/` directory (the marker for a KnowledgeStore-style root) and
uses `references/references.bib` inside it. Pass `--bib <path>` to override.

## Usage

```bash
typst-ref KnowledgeStore/Research/Notes.typ
typst-ref --dry-run --json KnowledgeStore/Research/Notes.typ
typst-ref --bib /path/to/references.bib KnowledgeStore/Research/Notes.typ
```

The input file argument is required and must be a real path (not `-`/stdin):
this tool edits two files in place (the `.typ` file and `references.bib`),
so there's no meaningful stdin mode.

Flags: `--json`, `--describe`, `--dry-run`, `--bib`.

Exit codes: `0` success, `1` tool error, `2` usage error.

See [../AGENTS.md](../AGENTS.md) for the conventions this tool follows.
