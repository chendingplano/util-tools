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

If this run rewrote any citations and the file has no `#bibliography(...)`
call yet, one is appended at the end (path relative to the `.typ` file).
Without it, Typst treats `@slug` as a label reference rather than a
citation and errors with "label `<slug>` does not exist in the document".

## Locating references.bib

By default, `typst-ref` walks up from the input file's directory looking for
a `references/` directory (the marker for a KnowledgeStore-style root) and
uses `references/references.bib` inside it. Pass `--bib <path>` to override.

## Usage

```bash
# Process the file 'KnowledgeStore/Research/Notes.typ.
# Command is invoked in the parent directory of 'Knowledgestore'
typst-ref KnowledgeStore/Research/Notes.typ

# '--dry-run' runs the command without actually modifying
# the files and 'references.bib'
typst-ref --dry-run --json KnowledgeStore/Research/Notes.typ

# Same as the first command, except that it
# use '--bib' to specify where the big file, overriding the default big file.
typst-ref --bib /path/to/references.bib KnowledgeStore/Research/Notes.typ
```

The input file argument is required and must be a real path (not `-`/stdin):
this tool edits two files in place (the `.typ` file and `references.bib`),
so there's no meaningful stdin mode.

In that example, `/path/to/references.bib` is an absolute path (it starts with /) — that's 
just illustrative placeholder text in the README, not tied to `KnowledgeStore` or the `.typ` 
file's location.

More generally: --bib is passed straight through to os.ReadFile/write, so:

If you give it an absolute path, that's used as-is.
If you give it a relative path, it's resolved relative to your shell's current working directory 
when you run the command — not relative to the `.typ` file, and not relative to KnowledgeStore.
So `typst-ref --bib references/references.bib KnowledgeStore/Research/Notes.typ` would look for 
`references/references.bib` under wherever you invoked the command from, which is easy to get 
wrong if you're not sitting in KnowledgeStore. Safer to either pass `--bib` as an absolute path, 
or just omit `--bib` entirely and let the tool auto-discover it by walking up from the `.typ` 
file's directory (the default behavior), which is relative to the file, not the CWD.

Flags: `--json`, `--describe`, `--dry-run`, `--bib`.
1. --json — emit a machine-readable JSON report instead of the human-readable text lines. 
   Contains file, bib, dry_run, section_found, section_removed, added (new bib entries), 
   already_present, replaced (citation count), and warnings.
2. --describe — print the tool's JSON descriptor (name, summary, keywords, args, examples) 
   and exit 0, without processing any file. Used by tool-index for discovery.
3. --dry-run — compute and report what would change (new bib entries, citation replacements, 
   section removal) without writing to either the .typ file or references.bib. Output is prefixed 
   [dry-run] in text mode, or "dry_run": true in JSON mode.
4. --bib <path> — override where the bibliography lives, instead of the default behavior of 
   walking up from the input file's directory looking for a references/ subdirectory and using 
   references/references.bib inside it.

Exit codes: `0` success, `1` tool error, `2` usage error.

See [../AGENTS.md](../AGENTS.md) for the conventions this tool follows.
