# util-tools — Conventions Contract

Rules for building a tool in this repo. This file is authoritative; skills and
READMEs point here rather than restating these rules.

## Repository shape

One self-contained Go module per tool, in its own directory:

```
<tool-name>/
├── go.mod              # module github.com/chendingplano/util-tools/<tool-name>
├── cmd/<tool-name>/    # thin main: flags, I/O, exit codes. NO LOGIC.
├── internal/           # the actual logic, as pure functions
├── testdata/           # golden files
└── README.md
```

**Directory name = module path suffix = binary name.** Always. A tool named
`typst-ref` lives in `typst-ref/`, has module path
`github.com/chendingplano/util-tools/typst-ref`, and installs as `typst-ref`.

`_template/` is the scaffold for new tools. Its leading underscore makes the Go
toolchain ignore it in `./...` patterns.

## The cmd/ rule

`cmd/<name>/main.go` parses flags, wires input and output, formats results and
sets exit codes. It contains no domain logic.

The library package takes `io.Reader`s and values, returns values and errors.
It prints nothing, reads no flags, and never calls `os.Exit`.

This is what lets a project (e.g. ChenWeb) import a tool's library directly and
wrap it in an HTTP handler instead of shelling out to a binary. Tools ship no
web code of their own.

## Required flags

| Flag | Rule |
|---|---|
| `--json` | Emit machine-readable JSON, including for errors. Mandatory on any tool producing structured output. |
| `--describe` | Emit the tool's JSON descriptor (see below) and exit 0. Mandatory on every tool. |
| `--dry-run` | Mandatory on any tool that edits files in place. |

Input: accept a file path argument, and read stdin when the argument is absent
or is `-`.

Errors: human-readable to stderr, non-zero exit. Never leave a partial write in
a destination file — write to a temp file in the destination's directory and
rename.

Exit codes: `0` success, `1` tool error, `2` usage error.

## The descriptor (`--describe`)

Metadata lives next to the code so it cannot drift from behaviour:

```json
{
  "name": "example-tool",
  "summary": "One line, imperative, what the tool does",
  "keywords": ["searchable", "terms"],
  "args": [
    {"name": "file", "type": "path", "required": false, "help": "Input file; reads stdin when absent or -"},
    {"name": "--out", "type": "path", "default": "out.txt", "help": "Destination"}
  ],
  "examples": ["example-tool input.txt --out result.txt"]
}
```

`name` MUST equal the binary name. `summary` MUST be one line. `keywords` is
what `tool-index search` matches against, so include the words a user would
actually type.

## Cross-platform rules

Tools must run on macOS, Linux and Windows. CI enforces this on every push.

- `path/filepath`, never `path`, for filesystem paths
- `os.TempDir()`, never a literal `/tmp`
- normalize CRLF on read — input authored on Windows must parse identically
- never shell out to `sed`, `grep`, `find`, or other Unix utilities
- do not assume case-sensitive paths or that symlinks exist
- write files `0644`, directories `0755`; do not reason about ownership

## Dependencies

Standard library by default. `github.com/pelletier/go-toml/v2` where TOML is
parsed (matching `shared/go`). A tool may depend on
`github.com/chendingplano/shared/go` when it genuinely needs logging, database
pools or auth. Nothing in this repo may be depended upon by `shared/go`.

Do not add a CLI framework.

## Testing

Table-driven unit tests against the library packages, with golden files in
`testdata/`. One smoke test per `cmd/`. TDD: the failing test comes first.

## Agent exposure

A tool is invisible to agents until opted in via `tools.toml`. New tools are
registered as `expose = "off"`, `mode = "ask"`. See `README.md` for the
policy format.

`tool-index` — the tool that grants permissions, via `sync-permissions
--write` — must never itself be `mode = "auto"`: an agent allowed to run it
unprompted could use it to grant itself further, arbitrary permissions.
`tools.toml` setting `tool-index` to `auto` fails to load.

## Version control

Commit with `jj commit`. Never `git commit`. Do not create branches.
