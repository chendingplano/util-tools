# Design: `util-tools` — a personal system-tool toolbox

- **Date:** 2026-09-19
- **Status:** Approved (design); not yet implemented
- **Repo:** `chendingplano/util-tools`, checked out at `~/Workspace/Utils`

## 1. Purpose

A home for small, project-independent command-line utilities — the kind of thing
that operates on files and documents and gets used from any directory, like `ls`
or `grep`. The motivating example is a tool that extracts references from a Typst
file into a `.bib` and rewrites the citations in Typst syntax, but the toolbox is
expected to accumulate many such tools over time.

Two properties drive every decision below:

1. The tools are **not tied to any project**. They must not depend on `tax`,
   `ChenWeb`, or any application's configuration or database to do their work.
2. The tools must be usable **by agents as well as by a human**, but agents must
   discover them incrementally rather than having the whole catalog pushed into
   context.

## 2. Decisions

### 2.1 Separate repository, not `shared/`

`Utils/` becomes its own git repository (initialized through `jj`), rather than
living in the `shared/` repo or in `ChenWeb`.

Rationale: `shared/go` is a library that `tax`, `ChenWeb` and `Kratos` all
`require`. Anything added to its `go.mod` enters all three module graphs
permanently. A toolbox that will accumulate dozens of narrow dependencies
(bibliography parsers, document lexers, HTTP clients for metadata lookup) does
not belong on that dependency path. A separate repo keeps the edge one-way: a
tool may depend on `shared/go`, nothing depends back.

The workspace root `.gitignore` already lists `Utils/`, consistent with it being
its own repository.

### 2.2 One Go module per tool

Each tool is a self-contained Go module in its own directory:

```
Utils/
├── AGENTS.md            # conventions contract (source of truth for agents)
├── CLAUDE.md            # symlink -> AGENTS.md
├── README.md            # human-facing: what exists, how to install and use
├── mise.toml            # install-all / build-all / test-all
├── tools.toml           # agent exposure policy (committed defaults)
├── tools.local.toml     # per-machine overrides (gitignored)
├── .github/workflows/   # cross-platform CI
├── docs/                # design and requirement documents
├── _template/           # scaffold copied when creating a new tool
├── mitmproxy/           # pre-existing Python utility, left as-is
├── tool-index/
│   ├── go.mod           # module github.com/chendingplano/util-tools/tool-index
│   ├── cmd/tool-index/
│   ├── internal/
│   ├── testdata/
│   └── README.md
└── typst-ref/           # (future — see §7)
```

**Directory name = module path suffix = binary name**, always.

Module paths are repo path plus subdirectory, e.g.
`github.com/chendingplano/util-tools/typst-ref`. Publishing a version uses a
path-prefixed tag (`typst-ref/v0.1.0`). Locally this is invisible because the
modules are listed in the workspace `go.work`.

`_template/` is prefixed with `_` so the Go toolchain ignores it and build tasks
skip it without special-casing.

### 2.3 Language: Go, with a Python escape hatch

Go, for single static binaries with no interpreter or virtualenv drift, native
cross-compilation, and zero-friction reuse of `shared/go` when a tool needs
logging, database pools or auth.

Where a library only exists in Python, the implementation may live in a
`pyimpl/` subdirectory invoked by a Go wrapper over a JSON-on-stdin/stdout
contract. The CLI surface is identical either way; callers cannot tell.

### 2.4 `cmd/` contains no logic

Every tool is a library package plus a thin `main`:

- The library takes `io.Reader`/values, returns values and errors. It prints
  nothing, reads no flags, and never calls `os.Exit`.
- `cmd/<name>/main.go` parses flags, wires I/O, formats output, sets exit codes.

This is what allows a project (e.g. `ChenWeb`) to import a tool's library
directly and wrap it in an HTTP handler, instead of shelling out to a binary.
Projects that do so write their own thin handler and UI; the tools module ships
no web code.

### 2.5 Binaries live in `$(go env GOPATH)/bin`

Installation is `go install ./cmd/...` per module, which places binaries in
`$(go env GOBIN)`, falling back to `$(go env GOPATH)/bin`.

Rationale: it is Go's own mechanism, works identically on macOS, Linux and
Windows (including the `.exe` suffix), and invents no new directory convention.
The repo never grows a `bin/` to gitignore.

**Machine-specific correction (verified 2026-09-19):** on this machine
`GOPATH` is `/Users/cding/.local/share/go` (not `~/go`), `GOBIN` is unset, and
that `bin` directory — which already contains `dlv`, `goose` and `gosec` — is
**not** on `PATH`. Meanwhile `PATH` contains `~/go/bin` twice, and that
directory does not exist. Setup therefore requires adding
`$(go env GOPATH)/bin` to `PATH` in the shell profile and removing the two dead
`~/go/bin` entries. This is a one-time, per-machine step, not a per-tool one.

`~/Workspace/bin` is deleted as part of this work; it is not on `PATH` at all,
and its only content is a `gosec` binary that `go install` can replace.

### 2.6 Cross-platform is a hard requirement

Tools must run on macOS, Linux and Windows. Enforced as rules in `AGENTS.md`:

- use `path/filepath`, never `path`, for filesystem paths
- use `os.TempDir()`, never a literal `/tmp`
- normalize CRLF line endings on read — text input authored on Windows must
  parse identically
- never shell out to `sed`, `grep`, `find` or other Unix utilities
- do not assume case-sensitive paths or the availability of symlinks
- write files `0644` / directories `0755`; do not reason about ownership

Because development happens on macOS, these claims are verified by CI rather
than asserted: a GitHub Actions matrix runs `go build` and `go test` for every
module on `ubuntu-latest`, `macos-latest` and `windows-latest`.

## 3. Tool conventions

These are the contract in `AGENTS.md`. Every tool obeys all of them.

| Convention | Rule |
|---|---|
| Input | Accept a file argument; read stdin when the argument is absent or `-` |
| Structured output | `--json` emits machine-readable JSON, including for errors |
| Self-description | `--describe` emits a JSON schema of the tool (see §4.1) |
| Mutation safety | Any tool that edits files in place supports `--dry-run` |
| Errors | Human-readable to stderr, non-zero exit; never a partial write to a destination file |
| Exit codes | `0` success, `1` tool error, `2` usage error |

No shared helper module exists initially. Isolated modules cannot share an
`internal/` package, and a `toolkit` module is not justified by one tool. The
conventions live as a written contract, and are extracted into a real shared
module only when a third tool demonstrates the duplication is real.

## 4. Agent access

Agents run tools as ordinary commands. The design problem is not invocation but
**discovery**: the catalog must not be pushed into agent context wholesale, and
exposure must be controlled per tool.

### 4.1 `--describe`

Every tool emits its own metadata, so description cannot drift from behaviour:

```json
{
  "name": "typst-ref",
  "summary": "Extract Typst references into a .bib and rewrite citations",
  "keywords": ["typst", "bibtex", "references", "citations"],
  "args": [
    {"name": "file", "type": "path", "required": true},
    {"name": "--bib", "type": "path", "default": "references/references.bib"}
  ],
  "output": {"type": "object", "properties": {"added": {"type": "integer"}}}
}
```

### 4.2 `tool-index`

The first tool built. It is a **registry, not a dispatcher** — it cannot run
another tool:

```
tool-index search <keywords>    # names + summaries of matching exposed tools
tool-index describe <name>      # full --describe output for one tool
tool-index sync-permissions     # write Bash() allowlist entries for auto-mode tools
```

It builds its catalog by invoking `--describe` across installed binaries at
install time and caching the result, rather than spawning a process per query.

### 4.3 Exposure policy

Descriptive metadata comes from the binary. *Permission* to use it is policy,
kept in `Utils/tools.toml`, overridable per machine in a gitignored
`tools.local.toml`:

```toml
[typst-ref]
expose = "discoverable"   # "off" | "discoverable"
mode   = "ask"            # "ask" | "auto"
```

- `expose = "off"` — `tool-index search` never returns it. Invisible to agents;
  still usable by a human at a shell.
- `mode = "auto"` — `sync-permissions` emits a `Bash(typst-ref:*)` entry into
  `~/Workspace/.claude/settings.json`, so agents are not prompted.
- `mode = "ask"` — omitted from the allowlist; each invocation prompts.

**A newly created tool defaults to `expose = "off"`, `mode = "ask"`.** Exposure
is opted into deliberately, per tool; a new tool never grants itself access.

### 4.4 Skills

Two skills, sourced from `~/Workspace/.agents/skills/` and symlinked into both
`~/.claude/skills/` and `~/Workspace/.claude/skills/`, per the workspace
convention.

- **`sys-tools`** — discovery. Its description is deliberately generic, so what
  enters context is one skill description rather than N tool descriptions. The
  body instructs the agent to `tool-index search`, then `tool-index describe`,
  then run the tool. Context cost stays flat as the catalog grows.
- **`create-sys-tools`** — creation. A procedure, not a restatement of the
  rules: ask for the tool's name, reject names colliding with existing commands,
  copy `_template/`, rewrite the module path, register in `go.work` and
  `tools.toml` (as `off`/`ask`), write a failing test, then the library, then
  `cmd/`. It points at `AGENTS.md` for the conventions rather than duplicating
  them, so the two cannot diverge.

### 4.5 MCP

Not built now. `--describe` is what keeps it cheap later: a single generic MCP
server can read `--describe` from every exposed binary and serve them all, with
no change to any tool.

## 5. Documentation split

- **`AGENTS.md`** — conventions contract: layout, cross-platform rules, flag
  conventions, TDD expectation, `go.work` wiring, naming. Source of truth.
  `CLAUDE.md` is a symlink to it.
- **`README.md`** — human-facing: the catalog, installation, usage.

## 6. Testing

Table-driven unit tests against the library packages, with golden files
(`testdata/in.typ` → `testdata/want.bib`), since the libraries are pure
functions. One smoke test per `cmd/`. No database in any test by default; a tool
that later needs one follows the existing `shared/go` test conventions.

CI runs the full matrix described in §2.6.

## 7. Out of scope

`typst-ref` is **not** part of this work. Its semantics are genuinely ambiguous
— extracting already-marked `@key` citations and filling in missing `.bib`
entries is a mechanical problem; recognizing references written as prose,
matching them to bibliographic records and rewriting them is a much larger and
error-prone one. It gets its own requirement document and its own design pass.

## 8. Deliverables

1. `Utils/` initialized as a `jj` repository with this document as its first commit
2. Repo scaffold: `AGENTS.md`, `CLAUDE.md` symlink, `README.md`, `mise.toml`,
   `.gitignore`, `_template/`
3. `tool-index`, dogfooding every convention in §3
4. `tools.toml` and `sync-permissions`
5. GitHub Actions matrix for ubuntu / macos / windows
6. `go.work` wiring; delete `~/Workspace/bin`; PATH fix for `$(go env GOPATH)/bin`
7. Skills `sys-tools` and `create-sys-tools`, with their symlinks
