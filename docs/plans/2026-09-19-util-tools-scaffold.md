# util-tools Toolbox Scaffold — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the `util-tools` repository — conventions, build/install workflow, the `tool-index` registry, exposure policy, CI, and the two agent skills — so that adding a new system tool afterwards is a mechanical, well-governed act.

**Architecture:** `Utils/` is a git repo (managed via `jj`) holding one self-contained Go module per tool. Each tool is a pure library package plus a thin `cmd/` main, and must implement `--json` and `--describe`. `tool-index` is a registry (not a dispatcher) that reads a `tools.toml` policy file, invokes `--describe` on exposed binaries, caches a catalog, and answers `search`/`describe` queries so agents discover tools incrementally instead of loading the whole catalog into context.

**Tech Stack:** Go 1.25, `github.com/pelletier/go-toml/v2` (matching `shared/go`), stdlib `flag` (no cobra — three subcommands do not justify the dependency), `mise` task runner, GitHub Actions, `jj` for version control.

**Spec:** `Utils/docs/2026-09-19-util-tools-design.md`

## Global Constraints

- **Repo:** `chendingplano/util-tools`, checked out at `/Users/cding/Workspace/Utils`.
- **Module paths:** `github.com/chendingplano/util-tools/<tool-dir>`. Directory name = module suffix = binary name, always.
- **Go directive:** `go 1.25.0` in every `go.mod` (matches `shared/go`).
- **Version control:** commit with `jj commit`, NEVER `git commit`. Raw `git` is read-only inspection only. Do not create branches or bookmarks.
- **Dependencies:** stdlib only, except `github.com/pelletier/go-toml/v2` where TOML is parsed. Do not add cobra, viper, or a CLI framework.
- **Cross-platform (all three of macOS, Linux, Windows):** use `path/filepath` never `path`; `os.TempDir()` never a literal `/tmp`; normalize CRLF on read; never shell out to `sed`/`grep`/`find`; do not assume case-sensitive paths or symlink availability; write files `0644`, directories `0755`.
- **Exit codes:** `0` success, `1` tool error, `2` usage error.
- **Out of scope:** `typst-ref` is NOT built by this plan. Do not create it.
- **Verified machine facts (do not re-derive, do not re-investigate):** `go env GOPATH` = `/Users/cding/.local/share/go`, holding the pre-existing `dlv`, `goose`, `gosec`, and NOT on `PATH`. `~/go/bin` is on `PATH` twice — placed there by nix home-manager at `nix/modules/home/home.nix:17,84-87` — but does not exist. `~/.zshrc` sets no Go PATH entries. `~/Workspace/bin` is not on `PATH` and holds only `gosec`.
- **Install target decided by the user:** `go env -w GOBIN="$HOME/go/bin"`, so installs land where nix already points. Do NOT edit `~/.zshrc`, and do NOT edit the nix configuration.

---

### Task 1: Repo scaffold, conventions contract, and install path

Establishes the repo's documentation and build surface, and fixes the PATH problem so `go install` produces runnable commands. No Go code yet, so no test cycle — the deliverable is verified by running the mise tasks and confirming a `go install`ed binary resolves on PATH.

**Files:**
- Create: `Utils/.gitignore`
- Create: `Utils/AGENTS.md`
- Create: `Utils/CLAUDE.md` (symlink → `AGENTS.md`)
- Create: `Utils/README.md`
- Create: `Utils/mise.toml`
- Modify: `~/.zshrc` (PATH entries)
- Delete: `/Users/cding/Workspace/bin`

**Interfaces:**
- Consumes: nothing.
- Produces: `mise install-all`, `mise build-all`, `mise test-all`, `mise fmt`; the rule set in `AGENTS.md` that every later task cites.

- [ ] **Step 1: Create `Utils/.gitignore`**

```gitignore
# Per-machine policy override
tools.local.toml

# Cross-compiled release artifacts
dist/

# OS noise
.DS_Store
```

- [ ] **Step 2: Create `Utils/AGENTS.md` — the conventions contract**

This file is the single source of truth for how tools are built. The `create-sys-tools` skill points at it and must not restate it.

```markdown
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
    {"name": "file", "type": "path", "required": true, "help": "Input file, or - for stdin"},
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

## Version control

Commit with `jj commit`. Never `git commit`. Do not create branches.
```

- [ ] **Step 3: Create `Utils/CLAUDE.md` as a symlink**

```bash
ln -s AGENTS.md CLAUDE.md
```

- [ ] **Step 4: Create `Utils/README.md` — human-facing**

```markdown
# util-tools

Small, project-independent command-line utilities. Each tool is a standalone Go
module that installs as an ordinary command, usable from any directory.

Agents building a tool here must follow [AGENTS.md](AGENTS.md).

## Install

Requires Go 1.25+ and `mise`.

```bash
mise install-all      # go install ./cmd/... in every tool module
```

Binaries land in `$(go env GOBIN)`. On this machine that is set to
`~/go/bin`, which is already on `PATH`:

```bash
go env -w GOBIN="$HOME/go/bin"   # one-time, per machine
```

On a fresh machine without that setting, binaries land in
`$(go env GOPATH)/bin` instead — put whichever one `go env GOBIN` reports on
your `PATH`.

## Tasks

| Task | Does |
|---|---|
| `mise install-all` | Install every tool to the Go bin directory |
| `mise build-all` | Cross-compile release artifacts into `dist/` |
| `mise test-all` | Run `go test ./...` in every module, including `_template` |
| `mise fmt` | `gofmt -w` across all modules |

## Tools

| Tool | Does |
|---|---|
| `tool-index` | Registry: discover and describe the other tools |
| `mitmproxy/` | Python capture helper (not a Go tool) |

## Agent exposure policy

Tools are invisible to agents until opted in. `tools.toml` holds the committed
defaults; `tools.local.toml` (gitignored) overrides them per machine:

```toml
[tool-index]
expose = "discoverable"   # "off" | "discoverable"
mode   = "ask"            # "ask" | "auto"
```

- `expose = "off"` — `tool-index search` never returns it. Invisible to agents,
  still usable by a human at a shell.
- `mode = "auto"` — `tool-index sync-permissions` emits a `Bash(<name>:*)`
  allowlist entry so agents are not prompted.
- `mode = "ask"` — omitted from the allowlist; each invocation prompts.

New tools are registered `off` / `ask`. Exposure is opted into deliberately.

`tool-index` locates `tools.toml` by checking, in order: `$UTIL_TOOLS_CONFIG`,
then `$UTIL_TOOLS_HOME/tools.toml`, then
`<os.UserConfigDir()>/util-tools/tools.toml`.

`mise.toml` sets `UTIL_TOOLS_HOME` to this repo, so mise tasks work with no
setup. To use `tool-index` against this repo's policy from an ordinary shell,
export it yourself:

```bash
export UTIL_TOOLS_HOME="$HOME/Workspace/Utils"
```
```

- [ ] **Step 5: Create `Utils/mise.toml`**

Each tool is its own module, so `./...` cannot cross module boundaries — the tasks loop over directories containing a `go.mod`. `install-all` skips `_`-prefixed dirs; `test-all` includes them so the template cannot rot.

```toml
[tools]
go = "1.25"

# Makes the repo self-contained: tool-index finds tools.toml without the user
# having to export anything. Outside mise it falls back to the user config dir.
[env]
UTIL_TOOLS_HOME = "{{config_root}}"

[tasks.install-all]
description = "Install every tool to the Go bin directory"
run = '''
set -e
for d in */; do
  case "$d" in _*) continue;; esac
  [ -f "$d/go.mod" ] || continue
  echo "==> installing $d"
  (cd "$d" && go install ./cmd/...)
done
'''

[tasks.test-all]
description = "Run tests in every module, including _template"
run = '''
set -e
for d in */ _*/; do
  [ -f "$d/go.mod" ] || continue
  echo "==> testing $d"
  (cd "$d" && go test ./...)
done
'''

[tasks.build-all]
description = "Cross-compile release artifacts into dist/"
run = '''
set -e
mkdir -p dist
for d in */; do
  case "$d" in _*) continue;; esac
  [ -f "$d/go.mod" ] || continue
  name="${d%/}"
  for platform in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64; do
    os="${platform%/*}"; arch="${platform#*/}"
    ext=""; [ "$os" = "windows" ] && ext=".exe"
    echo "==> $name $os/$arch"
    (cd "$d" && GOOS="$os" GOARCH="$arch" go build -o "../dist/${name}_${os}_${arch}${ext}" ./cmd/...)
  done
done
'''

[tasks.fmt]
description = "gofmt -w across all modules"
run = '''
set -e
for d in */ _*/; do
  [ -f "$d/go.mod" ] || continue
  (cd "$d" && gofmt -w .)
done
'''
```

- [ ] **Step 6: Make installed binaries reachable via GOBIN**

**Do not edit `~/.zshrc`, and do not edit the nix config.** Verified: `~/.zshrc`
contains no Go PATH entries. PATH is declaratively managed by nix home-manager
at `/Users/cding/Workspace/nix/modules/home/home.nix` (lines 17 and 84-87),
which already places `$HOME/go/bin` on PATH twice — but that directory does not
exist, which is why `go install` currently produces unreachable binaries.

The user chose to point `GOBIN` at the directory nix already exports, which
takes effect immediately with no rebuild and no shell restart:

```bash
go env -w GOBIN="$HOME/go/bin"
```

Verify:

```bash
go env GOBIN
```
Expected: `/Users/cding/go/bin`.

```bash
zsh -lc 'echo $PATH' | tr ":" "\n" | grep -c "$HOME/go/bin"
```
Expected: `2` or more — nix already puts it there; nothing needs adding.

Note that `$(go env GOPATH)/bin` (`/Users/cding/.local/share/go/bin`) keeps the
pre-existing `dlv`, `goose` and `gosec`. That split is accepted and recorded;
do not attempt to consolidate it.

- [ ] **Step 7: Delete `~/Workspace/bin`**

Confirm the contents are replaceable before deleting (it should contain only `gosec`, which `go install` can restore):

```bash
ls -la /Users/cding/Workspace/bin
rm -rf /Users/cding/Workspace/bin
```

- [ ] **Step 8: Commit**

```bash
cd /Users/cding/Workspace/Utils
jj commit -m "Add repo scaffold: conventions contract, README, mise tasks

AGENTS.md is the authoritative conventions contract (CLAUDE.md symlinks
to it); README.md is human-facing. mise tasks loop over per-tool modules
since ./... cannot cross module boundaries.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 2: `_template` — the scaffold new tools are copied from

A working, tested, minimal tool that demonstrates every convention. It is built and tested in CI so it cannot rot. `create-sys-tools` copies it.

**Files:**
- Create: `Utils/_template/go.mod`
- Create: `Utils/_template/README.md`
- Create: `Utils/_template/internal/example/example.go`
- Test: `Utils/_template/internal/example/example_test.go`
- Create: `Utils/_template/cmd/example-tool/main.go`
- Test: `Utils/_template/cmd/example-tool/main_test.go`

**Interfaces:**
- Consumes: the rules in `AGENTS.md` (Task 1).
- Produces: `descriptor` JSON shape used by Task 3's parser; the `Count(io.Reader) (Result, error)` / `Result{Lines, Words int}` pattern that `create-sys-tools` rewrites.

- [ ] **Step 1: Write the failing test for the library**

`_template/internal/example/example_test.go` — note the CRLF case, which is the cross-platform rule most likely to be violated:

```go
package example

import (
	"strings"
	"testing"
)

func TestCount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Result
	}{
		{"empty", "", Result{Lines: 0, Words: 0}},
		{"single line no newline", "hello world", Result{Lines: 1, Words: 2}},
		{"trailing newline", "a b\n", Result{Lines: 1, Words: 2}},
		{"two lines", "a\nb c\n", Result{Lines: 2, Words: 3}},
		{"crlf line endings", "a\r\nb c\r\n", Result{Lines: 2, Words: 3}},
		{"blank lines counted", "a\n\nb\n", Result{Lines: 3, Words: 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Count(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("Count() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Count() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Create the module and run the test to verify it fails**

```bash
cd /Users/cding/Workspace/Utils/_template
go mod init github.com/chendingplano/util-tools/_template
go test ./...
```
Expected: FAIL — `undefined: Count`, `undefined: Result`.

- [ ] **Step 3: Implement the library**

`_template/internal/example/example.go`:

```go
// Package example is the reference implementation for a util-tools library
// package: pure functions, no flags, no printing, no os.Exit.
package example

import (
	"bufio"
	"io"
	"strings"
)

// Result is the structured output of Count.
type Result struct {
	Lines int `json:"lines"`
	Words int `json:"words"`
}

// Count reports the number of lines and whitespace-separated words in r.
// CRLF line endings are normalized, so input authored on Windows counts
// identically to input authored on Unix.
func Count(r io.Reader) (Result, error) {
	var res Result
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSuffix(sc.Text(), "\r")
		res.Lines++
		res.Words += len(strings.Fields(line))
	}
	if err := sc.Err(); err != nil {
		return Result{}, err
	}
	return res, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
cd /Users/cding/Workspace/Utils/_template && go test ./...
```
Expected: PASS.

- [ ] **Step 5: Write the failing smoke test for `cmd/`**

`_template/cmd/example-tool/main_test.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunJSON(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"--json", "-"}, strings.NewReader("a b\nc\n"), &out, &errOut)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	var got struct {
		Lines int `json:"lines"`
		Words int `json:"words"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v (%q)", err, out.String())
	}
	if got.Lines != 2 || got.Words != 3 {
		t.Errorf("got %+v, want lines=2 words=3", got)
	}
}

func TestRunDescribe(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"--describe"}, strings.NewReader(""), &out, &errOut)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	var d struct {
		Name     string   `json:"name"`
		Summary  string   `json:"summary"`
		Keywords []string `json:"keywords"`
	}
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatalf("descriptor is not JSON: %v", err)
	}
	if d.Name != "example-tool" {
		t.Errorf("descriptor name = %q, want %q", d.Name, "example-tool")
	}
	if d.Summary == "" || len(d.Keywords) == 0 {
		t.Errorf("descriptor missing summary or keywords: %+v", d)
	}
}

func TestRunUsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"--nope"}, strings.NewReader(""), &out, &errOut)
	if code != 2 {
		t.Errorf("run() = %d, want 2 for usage error", code)
	}
}
```

- [ ] **Step 6: Run it to verify it fails**

```bash
cd /Users/cding/Workspace/Utils/_template && go test ./cmd/...
```
Expected: FAIL — `undefined: run`.

- [ ] **Step 7: Implement `cmd/example-tool/main.go`**

Note the shape: `run` is testable (takes args and streams, returns an exit code); `main` is four lines. Copy this structure for every tool.

```go
// Command example-tool is the reference implementation of a util-tools CLI.
// It parses flags, wires I/O and sets exit codes; all logic lives in
// internal/example.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/chendingplano/util-tools/_template/internal/example"
)

const descriptor = `{
  "name": "example-tool",
  "summary": "Count lines and words in a file or stdin",
  "keywords": ["count", "lines", "words", "text", "example"],
  "args": [
    {"name": "file", "type": "path", "required": true, "help": "Input file, or - for stdin"},
    {"name": "--json", "type": "bool", "help": "Emit JSON instead of text"}
  ],
  "examples": ["example-tool notes.txt", "cat notes.txt | example-tool -"]
}`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes the tool. It returns 0 on success, 1 on a tool error and 2 on
// a usage error.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("example-tool", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit JSON instead of text")
	describe := fs.Bool("describe", false, "print this tool's JSON descriptor and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *describe {
		fmt.Fprintln(stdout, descriptor)
		return 0
	}

	in := stdin
	if fs.NArg() > 0 && fs.Arg(0) != "-" {
		f, err := os.Open(fs.Arg(0))
		if err != nil {
			fmt.Fprintf(stderr, "example-tool: %v\n", err)
			return 1
		}
		defer f.Close()
		in = f
	}

	res, err := example.Count(in)
	if err != nil {
		fmt.Fprintf(stderr, "example-tool: %v\n", err)
		return 1
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fmt.Fprintf(stderr, "example-tool: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "%d lines, %d words\n", res.Lines, res.Words)
	return 0
}
```

- [ ] **Step 8: Run the full module test suite**

```bash
cd /Users/cding/Workspace/Utils/_template && go test ./...
```
Expected: PASS, all tests.

- [ ] **Step 9: Create `_template/README.md`**

```markdown
# _template

Scaffold for a new util-tools tool. The `create-sys-tools` skill copies this
directory and rewrites the names.

It is a working tool (`example-tool` counts lines and words) and is tested in
CI, so the conventions it demonstrates cannot silently rot.

To create a tool by hand:

1. `cp -R _template <tool-name>` and `cd <tool-name>`
2. Rewrite the module path in `go.mod` to
   `github.com/chendingplano/util-tools/<tool-name>`
3. Rename `cmd/example-tool/` to `cmd/<tool-name>/` and rename
   `internal/example/` to something meaningful
4. Update the `descriptor` constant — `name` MUST equal the binary name
5. Add the module to `/Users/cding/Workspace/go.work`
6. Register it in `tools.toml` as `expose = "off"`, `mode = "ask"`
7. Write the failing test first

The rules are in [../AGENTS.md](../AGENTS.md).
```

- [ ] **Step 10: Commit**

```bash
cd /Users/cding/Workspace/Utils
jj commit -m "Add _template: the reference tool scaffold

Working, tested example-tool demonstrating every convention: pure
library, thin testable run(), --json, --describe, CRLF normalization,
and the 0/1/2 exit code contract. Underscore prefix keeps it out of
./... patterns; test-all still exercises it so it cannot rot.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 3: `tool-index` module and the `describe` package

The descriptor type, shared by every tool's `--describe` output and by the registry that consumes it.

**Files:**
- Create: `Utils/tool-index/go.mod`
- Create: `Utils/tool-index/internal/describe/describe.go`
- Test: `Utils/tool-index/internal/describe/describe_test.go`

**Interfaces:**
- Consumes: the descriptor JSON shape emitted by `_template` (Task 2).
- Produces:
  - `type Arg struct { Name, Type, Help, Default string; Required bool }`
  - `type Descriptor struct { Name, Summary string; Keywords []string; Args []Arg; Examples []string }`
  - `func Parse(b []byte) (Descriptor, error)` — parses and validates
  - `func (d Descriptor) Validate() error`

- [ ] **Step 1: Write the failing test**

`tool-index/internal/describe/describe_test.go`:

```go
package describe

import "testing"

const valid = `{
  "name": "example-tool",
  "summary": "Count lines and words",
  "keywords": ["count", "text"],
  "args": [{"name": "file", "type": "path", "required": true}],
  "examples": ["example-tool a.txt"]
}`

func TestParseValid(t *testing.T) {
	d, err := Parse([]byte(valid))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if d.Name != "example-tool" {
		t.Errorf("Name = %q, want %q", d.Name, "example-tool")
	}
	if len(d.Keywords) != 2 {
		t.Errorf("Keywords = %v, want 2 entries", d.Keywords)
	}
	if len(d.Args) != 1 || !d.Args[0].Required {
		t.Errorf("Args = %+v, want one required arg", d.Args)
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"not json", `nonsense`},
		{"missing name", `{"summary":"x","keywords":["a"]}`},
		{"missing summary", `{"name":"x","keywords":["a"]}`},
		{"no keywords", `{"name":"x","summary":"y","keywords":[]}`},
		{"multiline summary", `{"name":"x","summary":"line one\nline two","keywords":["a"]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse([]byte(tt.in)); err == nil {
				t.Errorf("Parse(%q) = nil error, want error", tt.in)
			}
		})
	}
}
```

- [ ] **Step 2: Create the module and run the test to verify it fails**

```bash
mkdir -p /Users/cding/Workspace/Utils/tool-index && cd /Users/cding/Workspace/Utils/tool-index
go mod init github.com/chendingplano/util-tools/tool-index
go test ./...
```
Expected: FAIL — `undefined: Parse`.

- [ ] **Step 3: Implement `describe.go`**

```go
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
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
cd /Users/cding/Workspace/Utils/tool-index && go test ./...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/cding/Workspace/Utils
jj commit -m "Add tool-index module with descriptor parsing

describe.Descriptor is the contract between a tool's --describe output
and the registry. Validation enforces the AGENTS.md rules: name,
single-line summary, at least one keyword.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 4: Exposure policy (`tools.toml`)

Policy is deliberately separate from descriptors: *what a tool is* comes from the binary, *whether an agent may use it* comes from this file.

**Files:**
- Create: `Utils/tool-index/internal/policy/policy.go`
- Test: `Utils/tool-index/internal/policy/policy_test.go`
- Create: `Utils/tools.toml`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `type Expose string` with `ExposeOff = "off"`, `ExposeDiscoverable = "discoverable"`
  - `type Mode string` with `ModeAsk = "ask"`, `ModeAuto = "auto"`
  - `type Entry struct { Expose Expose; Mode Mode }`
  - `type Set map[string]Entry`
  - `func Load(dir string) (Set, error)` — reads `tools.toml` then overlays `tools.local.toml`
  - `func (s Set) Discoverable() []string` — sorted names
  - `func (s Set) AutoMode() []string` — sorted names of discoverable tools with `mode = "auto"`
  - `func FindDir() (string, error)` — `$UTIL_TOOLS_CONFIG` dir, else `$UTIL_TOOLS_HOME`, else `os.UserConfigDir()/util-tools`

- [ ] **Step 1: Write the failing test**

`tool-index/internal/policy/policy_test.go`:

```go
package policy

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "tools.toml", `
[tool-index]
expose = "discoverable"
mode = "auto"

[quiet-tool]
expose = "off"
mode = "ask"

[sparse-tool]
`)
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// An entry with no fields defaults to the safe values.
	if got := s["sparse-tool"]; got.Expose != ExposeOff || got.Mode != ModeAsk {
		t.Errorf("sparse-tool = %+v, want off/ask", got)
	}
	if got, want := s.Discoverable(), []string{"tool-index"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Discoverable() = %v, want %v", got, want)
	}
	if got, want := s.AutoMode(), []string{"tool-index"}; !reflect.DeepEqual(got, want) {
		t.Errorf("AutoMode() = %v, want %v", got, want)
	}
}

func TestLocalOverlayWins(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "tools.toml", "[a]\nexpose = \"off\"\nmode = \"ask\"\n[b]\nexpose = \"discoverable\"\nmode = \"ask\"\n")
	write(t, dir, "tools.local.toml", "[a]\nexpose = \"discoverable\"\nmode = \"auto\"\n")
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := s["a"]; got.Expose != ExposeDiscoverable || got.Mode != ModeAuto {
		t.Errorf("a = %+v, want discoverable/auto", got)
	}
	if got := s["b"]; got.Expose != ExposeDiscoverable || got.Mode != ModeAsk {
		t.Errorf("b = %+v, want discoverable/ask (untouched by overlay)", got)
	}
}

func TestLoadRejectsUnknownValues(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "tools.toml", "[a]\nexpose = \"maybe\"\n")
	if _, err := Load(dir); err == nil {
		t.Error("Load() = nil error, want error for unknown expose value")
	}
}

func TestAutoModeExcludesUnexposed(t *testing.T) {
	dir := t.TempDir()
	// mode=auto on an unexposed tool must never produce a permission grant.
	write(t, dir, "tools.toml", "[a]\nexpose = \"off\"\nmode = \"auto\"\n")
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := s.AutoMode(); len(got) != 0 {
		t.Errorf("AutoMode() = %v, want empty for an unexposed tool", got)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Error("Load() = nil error, want error when tools.toml is absent")
	}
}
```

- [ ] **Step 2: Add the TOML dependency and run the test to verify it fails**

```bash
cd /Users/cding/Workspace/Utils/tool-index
go get github.com/pelletier/go-toml/v2
go test ./internal/policy/...
```
Expected: FAIL — `undefined: Load`.

- [ ] **Step 3: Implement `policy.go`**

```go
// Package policy reads the agent-exposure policy for util-tools.
//
// Descriptive metadata about a tool comes from the tool binary itself
// (see package describe). This package answers only the policy question:
// may an agent see and run it.
package policy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

// Expose controls whether agents can discover a tool.
type Expose string

// Mode controls whether agents are prompted before running a tool.
type Mode string

const (
	ExposeOff          Expose = "off"
	ExposeDiscoverable Expose = "discoverable"

	ModeAsk  Mode = "ask"
	ModeAuto Mode = "auto"
)

// Entry is one tool's policy. The zero value is the safe default:
// invisible to agents, and prompted if run.
type Entry struct {
	Expose Expose `toml:"expose"`
	Mode   Mode   `toml:"mode"`
}

// Set maps tool name to policy.
type Set map[string]Entry

const (
	fileName      = "tools.toml"
	localFileName = "tools.local.toml"
)

// FindDir locates the directory holding tools.toml, checking
// $UTIL_TOOLS_CONFIG (a file path), then $UTIL_TOOLS_HOME, then the user
// config directory.
func FindDir() (string, error) {
	if p := os.Getenv("UTIL_TOOLS_CONFIG"); p != "" {
		return filepath.Dir(p), nil
	}
	if p := os.Getenv("UTIL_TOOLS_HOME"); p != "" {
		return p, nil
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("policy: cannot locate config dir: %w", err)
	}
	return filepath.Join(cfg, "util-tools"), nil
}

// Load reads tools.toml from dir and overlays tools.local.toml if present.
// A missing tools.toml is an error; a missing tools.local.toml is not.
func Load(dir string) (Set, error) {
	base, err := readFile(filepath.Join(dir, fileName))
	if err != nil {
		return nil, err
	}
	local, err := readFile(filepath.Join(dir, localFileName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for name, e := range local {
		base[name] = e
	}
	for name, e := range base {
		if err := e.validate(name); err != nil {
			return nil, err
		}
	}
	return base, nil
}

func readFile(path string) (Set, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("policy: reading %s: %w", path, err)
	}
	var s Set
	if err := toml.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("policy: parsing %s: %w", path, err)
	}
	for name, e := range s {
		if e.Expose == "" {
			e.Expose = ExposeOff
		}
		if e.Mode == "" {
			e.Mode = ModeAsk
		}
		s[name] = e
	}
	return s, nil
}

func (e Entry) validate(name string) error {
	switch e.Expose {
	case ExposeOff, ExposeDiscoverable:
	default:
		return fmt.Errorf("policy: tool %q: unknown expose %q (want %q or %q)",
			name, e.Expose, ExposeOff, ExposeDiscoverable)
	}
	switch e.Mode {
	case ModeAsk, ModeAuto:
	default:
		return fmt.Errorf("policy: tool %q: unknown mode %q (want %q or %q)",
			name, e.Mode, ModeAsk, ModeAuto)
	}
	return nil
}

// Discoverable returns the sorted names of tools agents may discover.
func (s Set) Discoverable() []string {
	return s.filter(func(e Entry) bool { return e.Expose == ExposeDiscoverable })
}

// AutoMode returns the sorted names of discoverable tools that run without
// prompting. An unexposed tool is never returned, whatever its mode.
func (s Set) AutoMode() []string {
	return s.filter(func(e Entry) bool {
		return e.Expose == ExposeDiscoverable && e.Mode == ModeAuto
	})
}

func (s Set) filter(keep func(Entry) bool) []string {
	var out []string
	for name, e := range s {
		if keep(e) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
cd /Users/cding/Workspace/Utils/tool-index && go test ./internal/policy/...
```
Expected: PASS.

- [ ] **Step 5: Create `Utils/tools.toml`**

`tool-index` is the one tool exposed by default — it is the entry point agents need in order to find anything else. It stays `ask` until you decide otherwise.

```toml
# Agent exposure policy. Committed defaults; override per machine in
# tools.local.toml (gitignored).
#
#   expose = "off" | "discoverable"   -- may agents discover this tool
#   mode   = "ask" | "auto"           -- may agents run it without prompting
#
# New tools are registered off/ask. Exposure is opted into deliberately.

[tool-index]
expose = "discoverable"
mode = "ask"
```

- [ ] **Step 6: Commit**

```bash
cd /Users/cding/Workspace/Utils
jj commit -m "Add exposure policy loading and tools.toml

Policy is separate from descriptors: the binary says what it is, this
file says whether an agent may see it. Defaults are off/ask, and
AutoMode() refuses to grant permission to an unexposed tool regardless
of its mode.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 5: Catalog — build, cache, search

The catalog invokes `--describe` across exposed binaries once at install time and caches the result, so `search` does not spawn a process per tool.

**Files:**
- Create: `Utils/tool-index/internal/catalog/catalog.go`
- Test: `Utils/tool-index/internal/catalog/catalog_test.go`

**Interfaces:**
- Consumes: `describe.Descriptor`, `describe.Parse` (Task 3); `policy.Set` (Task 4).
- Produces:
  - `type Runner func(name string) ([]byte, error)`
  - `type Catalog struct { Built time.Time; Tools []describe.Descriptor }`
  - `func Build(names []string, run Runner) (Catalog, []error)`
  - `func ExecRunner(name string) ([]byte, error)`
  - `func (c Catalog) Search(query []string) []describe.Descriptor`
  - `func (c Catalog) Get(name string) (describe.Descriptor, bool)`
  - `func Save(path string, c Catalog) error` / `func Load(path string) (Catalog, error)`
  - `func DefaultPath() (string, error)` — `os.UserCacheDir()/util-tools/catalog.json`

- [ ] **Step 1: Write the failing test**

`tool-index/internal/catalog/catalog_test.go`:

```go
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
```

- [ ] **Step 2: Run it to verify it fails**

```bash
cd /Users/cding/Workspace/Utils/tool-index && go test ./internal/catalog/...
```
Expected: FAIL — `undefined: Build`.

- [ ] **Step 3: Implement `catalog.go`**

```go
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

	var out []describe.Descriptor
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
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
cd /Users/cding/Workspace/Utils/tool-index && go test ./...
```
Expected: PASS, all packages.

- [ ] **Step 5: Commit**

```bash
cd /Users/cding/Workspace/Utils
jj commit -m "Add catalog: build from --describe, cache, and search

Build() tolerates missing or malformed tools so one broken binary cannot
blank the registry, and rejects a descriptor whose name disagrees with
its binary. Search ranks by number of matching query terms.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 6: Permission entry generation

Turns `mode = "auto"` into Claude Code allowlist entries. This writes to a large, hand-maintained settings file, so the default is to print; writing is opt-in and backed up.

**Files:**
- Create: `Utils/tool-index/internal/permissions/permissions.go`
- Test: `Utils/tool-index/internal/permissions/permissions_test.go`

**Interfaces:**
- Consumes: `policy.Set` (Task 4).
- Produces:
  - `func Entry(tool string) string` — `Bash(<tool>:*)`
  - `func Missing(tools []string, existing []string) []string`
  - `func ReadAllow(path string) ([]string, error)`
  - `func Apply(path string, add []string) error` — backs up, merges, writes atomically

- [ ] **Step 1: Write the failing test**

`tool-index/internal/permissions/permissions_test.go`:

```go
package permissions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const settings = `{
  "permissions": {
    "allow": ["Bash(ls:*)", "Bash(tool-index:*)"],
    "deny": ["Bash(rm:*)"]
  },
  "env": {"FOO": "bar"}
}`

func TestEntry(t *testing.T) {
	if got, want := Entry("typst-ref"), "Bash(typst-ref:*)"; got != want {
		t.Errorf("Entry() = %q, want %q", got, want)
	}
}

func TestMissing(t *testing.T) {
	existing := []string{"Bash(ls:*)", "Bash(tool-index:*)"}
	got := Missing([]string{"tool-index", "typst-ref"}, existing)
	want := []string{"Bash(typst-ref:*)"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Missing() = %v, want %v", got, want)
	}
}

func TestReadAllow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte(settings), 0o644)
	got, err := ReadAllow(path)
	if err != nil {
		t.Fatalf("ReadAllow() error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("ReadAllow() = %v, want 2 entries", got)
	}
}

func TestApplyPreservesOtherKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, []byte(settings), 0o644)

	if err := Apply(path, []string{"Bash(typst-ref:*)"}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	var got struct {
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
		} `json:"permissions"`
		Env map[string]string `json:"env"`
	}
	b, _ := os.ReadFile(path)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if len(got.Permissions.Allow) != 3 {
		t.Errorf("allow = %v, want 3 entries", got.Permissions.Allow)
	}
	if len(got.Permissions.Deny) != 1 {
		t.Errorf("deny lost: %v", got.Permissions.Deny)
	}
	if got.Env["FOO"] != "bar" {
		t.Errorf("env lost: %v", got.Env)
	}
}

func TestApplyWritesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, []byte(settings), 0o644)

	if err := Apply(path, []string{"Bash(typst-ref:*)"}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	b, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("backup not written: %v", err)
	}
	if string(b) != settings {
		t.Error("backup does not match the original file")
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, []byte(settings), 0o644)

	Apply(path, []string{"Bash(typst-ref:*)"})
	Apply(path, []string{"Bash(typst-ref:*)"})

	allow, _ := ReadAllow(path)
	count := 0
	for _, a := range allow {
		if a == "Bash(typst-ref:*)" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("entry appears %d times, want 1", count)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

```bash
cd /Users/cding/Workspace/Utils/tool-index && go test ./internal/permissions/...
```
Expected: FAIL — `undefined: Entry`.

- [ ] **Step 3: Implement `permissions.go`**

Top-level keys are held as `json.RawMessage` so nothing outside `permissions.allow` is re-encoded or reinterpreted.

```go
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
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
cd /Users/cding/Workspace/Utils/tool-index && go test ./...
```
Expected: PASS, all packages.

- [ ] **Step 5: Commit**

```bash
cd /Users/cding/Workspace/Utils
jj commit -m "Add permission entry generation for auto-mode tools

Carries unrelated settings keys through as raw JSON, backs up before
writing, writes atomically, and is idempotent.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 7: `cmd/tool-index` — the CLI

Wires the packages into `search`, `describe`, `sync-permissions` and `refresh`, plus `tool-index`'s own `--describe`.

**Files:**
- Create: `Utils/tool-index/cmd/tool-index/main.go`
- Test: `Utils/tool-index/cmd/tool-index/main_test.go`
- Create: `Utils/tool-index/README.md`

**Interfaces:**
- Consumes: `describe` (Task 3), `policy` (Task 4), `catalog` (Task 5), `permissions` (Task 6).
- Produces: the `tool-index` binary. Subcommands: `search <terms...>`, `describe <name>`, `refresh`, `sync-permissions --settings <path> [--write]`. Global: `--json`, `--describe`.

- [ ] **Step 1: Write the failing test**

`tool-index/cmd/tool-index/main_test.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// harness writes a policy dir and a cached catalog, and points the process at
// them via environment variables.
func harness(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "tools.toml"), []byte(`
[tool-index]
expose = "discoverable"
mode = "auto"

[hidden-tool]
expose = "off"
mode = "ask"
`), 0o644)
	cache := filepath.Join(dir, "catalog.json")
	os.WriteFile(cache, []byte(`{
      "built": "2026-09-19T00:00:00Z",
      "tools": [
        {"name":"tool-index","summary":"Discover and describe util-tools",
         "keywords":["registry","discover"]}
      ]
    }`), 0o644)
	t.Setenv("UTIL_TOOLS_HOME", dir)
	t.Setenv("UTIL_TOOLS_CATALOG", cache)
	return dir
}

func TestDescribeSelf(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--describe"}, &out, &errOut); code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	var d struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out.Bytes(), &d); err != nil {
		t.Fatalf("descriptor is not JSON: %v", err)
	}
	if d.Name != "tool-index" {
		t.Errorf("name = %q, want tool-index", d.Name)
	}
}

func TestSearchText(t *testing.T) {
	harness(t)
	var out, errOut bytes.Buffer
	if code := run([]string{"search", "registry"}, &out, &errOut); code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "tool-index") {
		t.Errorf("output = %q, want it to mention tool-index", out.String())
	}
}

func TestSearchJSON(t *testing.T) {
	harness(t)
	var out, errOut bytes.Buffer
	if code := run([]string{"--json", "search", "registry"}, &out, &errOut); code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	var got []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v (%q)", err, out.String())
	}
	if len(got) != 1 || got[0].Name != "tool-index" {
		t.Errorf("results = %+v, want one tool-index entry", got)
	}
}

func TestSearchNoMatchExitsZero(t *testing.T) {
	harness(t)
	var out, errOut bytes.Buffer
	if code := run([]string{"search", "kubernetes"}, &out, &errOut); code != 0 {
		t.Errorf("run() = %d, want 0 — no match is not an error", code)
	}
}

func TestDescribeToolNotFound(t *testing.T) {
	harness(t)
	var out, errOut bytes.Buffer
	if code := run([]string{"describe", "absent"}, &out, &errOut); code != 1 {
		t.Errorf("run() = %d, want 1", code)
	}
}

func TestSyncPermissionsDefaultsToDryRun(t *testing.T) {
	harness(t)
	settings := filepath.Join(t.TempDir(), "settings.json")
	original := `{"permissions":{"allow":[]}}`
	os.WriteFile(settings, []byte(original), 0o644)

	var out, errOut bytes.Buffer
	if code := run([]string{"sync-permissions", "--settings", settings}, &out, &errOut); code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Bash(tool-index:*)") {
		t.Errorf("output = %q, want the proposed entry printed", out.String())
	}
	b, _ := os.ReadFile(settings)
	if string(b) != original {
		t.Error("settings file was modified without --write")
	}
}

func TestSyncPermissionsWrite(t *testing.T) {
	harness(t)
	settings := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(settings, []byte(`{"permissions":{"allow":[]}}`), 0o644)

	var out, errOut bytes.Buffer
	if code := run([]string{"sync-permissions", "--settings", settings, "--write"}, &out, &errOut); code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, errOut.String())
	}
	b, _ := os.ReadFile(settings)
	if !strings.Contains(string(b), "Bash(tool-index:*)") {
		t.Errorf("settings = %s, want the entry added", b)
	}
	// hidden-tool is expose=off and must never be granted.
	if strings.Contains(string(b), "hidden-tool") {
		t.Error("an unexposed tool was granted a permission entry")
	}
}

func TestUnknownSubcommandIsUsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"frobnicate"}, &out, &errOut); code != 2 {
		t.Errorf("run() = %d, want 2", code)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

```bash
cd /Users/cding/Workspace/Utils/tool-index && go test ./cmd/...
```
Expected: FAIL — `undefined: run`.

- [ ] **Step 3: Implement `main.go`**

```go
// Command tool-index is the util-tools registry. It finds and describes the
// other tools; it deliberately cannot run them.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chendingplano/util-tools/tool-index/internal/catalog"
	"github.com/chendingplano/util-tools/tool-index/internal/describe"
	"github.com/chendingplano/util-tools/tool-index/internal/permissions"
	"github.com/chendingplano/util-tools/tool-index/internal/policy"
)

const descriptor = `{
  "name": "tool-index",
  "summary": "Discover and describe the util-tools system commands",
  "keywords": ["registry", "index", "discover", "tools", "utilities", "search"],
  "args": [
    {"name": "search", "type": "subcommand", "help": "Find tools matching keywords"},
    {"name": "describe", "type": "subcommand", "help": "Print one tool's full descriptor"},
    {"name": "refresh", "type": "subcommand", "help": "Rebuild the cached catalog"},
    {"name": "sync-permissions", "type": "subcommand", "help": "Emit allowlist entries for auto-mode tools"}
  ],
  "examples": [
    "tool-index search bibtex typst",
    "tool-index describe typst-ref",
    "tool-index sync-permissions --settings ~/Workspace/.claude/settings.json --write"
  ]
}`

const usage = `tool-index — the util-tools registry

Usage:
  tool-index search <terms...>                 find tools matching keywords
  tool-index describe <name>                   print one tool's full descriptor
  tool-index refresh                           rebuild the cached catalog
  tool-index sync-permissions --settings PATH [--write]

Flags:
  --json        emit JSON
  --describe    print this tool's descriptor and exit
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tool-index", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	asJSON := fs.Bool("json", false, "emit JSON")
	self := fs.Bool("describe", false, "print this tool's descriptor and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *self {
		fmt.Fprintln(stdout, descriptor)
		return 0
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return 2
	}

	switch fs.Arg(0) {
	case "search":
		return cmdSearch(fs.Args()[1:], *asJSON, stdout, stderr)
	case "describe":
		return cmdDescribe(fs.Args()[1:], *asJSON, stdout, stderr)
	case "refresh":
		return cmdRefresh(stdout, stderr)
	case "sync-permissions":
		return cmdSyncPermissions(fs.Args()[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "tool-index: unknown command %q\n\n", fs.Arg(0))
		fs.Usage()
		return 2
	}
}

// catalogPath honours UTIL_TOOLS_CATALOG so tests and unusual setups can
// redirect the cache.
func catalogPath() (string, error) {
	if p := os.Getenv("UTIL_TOOLS_CATALOG"); p != "" {
		return p, nil
	}
	return catalog.DefaultPath()
}

// loadCatalog reads the cache, rebuilding it if it is absent.
func loadCatalog(stderr io.Writer) (catalog.Catalog, error) {
	path, err := catalogPath()
	if err != nil {
		return catalog.Catalog{}, err
	}
	c, err := catalog.Load(path)
	if err == nil {
		return c, nil
	}
	return rebuild(path, stderr)
}

func rebuild(path string, stderr io.Writer) (catalog.Catalog, error) {
	dir, err := policy.FindDir()
	if err != nil {
		return catalog.Catalog{}, err
	}
	set, err := policy.Load(dir)
	if err != nil {
		return catalog.Catalog{}, err
	}
	c, errs := catalog.Build(set.Discoverable(), catalog.ExecRunner)
	for _, e := range errs {
		fmt.Fprintf(stderr, "tool-index: warning: %v\n", e)
	}
	if err := catalog.Save(path, c); err != nil {
		return catalog.Catalog{}, err
	}
	return c, nil
}

func cmdSearch(args []string, asJSON bool, stdout, stderr io.Writer) int {
	c, err := loadCatalog(stderr)
	if err != nil {
		fmt.Fprintf(stderr, "tool-index: %v\n", err)
		return 1
	}
	hits := c.Search(args)
	if asJSON {
		return writeJSON(stdout, stderr, hits)
	}
	if len(hits) == 0 {
		fmt.Fprintln(stdout, "no matching tools")
		return 0
	}
	width := 0
	for _, d := range hits {
		if len(d.Name) > width {
			width = len(d.Name)
		}
	}
	for _, d := range hits {
		fmt.Fprintf(stdout, "%-*s  %s\n", width, d.Name, d.Summary)
	}
	return 0
}

func cmdDescribe(args []string, asJSON bool, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "tool-index: describe requires exactly one tool name")
		return 2
	}
	c, err := loadCatalog(stderr)
	if err != nil {
		fmt.Fprintf(stderr, "tool-index: %v\n", err)
		return 1
	}
	d, ok := c.Get(args[0])
	if !ok {
		fmt.Fprintf(stderr, "tool-index: no exposed tool named %q\n", args[0])
		return 1
	}
	if asJSON {
		return writeJSON(stdout, stderr, d)
	}
	return writeHuman(stdout, stderr, d)
}

func writeHuman(stdout, stderr io.Writer, d describe.Descriptor) int {
	fmt.Fprintf(stdout, "%s — %s\n", d.Name, d.Summary)
	if len(d.Keywords) > 0 {
		fmt.Fprintf(stdout, "keywords: %s\n", strings.Join(d.Keywords, ", "))
	}
	if len(d.Args) > 0 {
		fmt.Fprintln(stdout, "\narguments:")
		for _, a := range d.Args {
			req := ""
			if a.Required {
				req = " (required)"
			}
			def := ""
			if a.Default != "" {
				def = fmt.Sprintf(" [default: %s]", a.Default)
			}
			fmt.Fprintf(stdout, "  %-12s %s%s%s%s\n", a.Name, a.Type, req, def, prefix(" — ", a.Help))
		}
	}
	if len(d.Examples) > 0 {
		fmt.Fprintln(stdout, "\nexamples:")
		for _, e := range d.Examples {
			fmt.Fprintf(stdout, "  %s\n", e)
		}
	}
	return 0
}

func prefix(p, s string) string {
	if s == "" {
		return ""
	}
	return p + s
}

func cmdRefresh(stdout, stderr io.Writer) int {
	path, err := catalogPath()
	if err != nil {
		fmt.Fprintf(stderr, "tool-index: %v\n", err)
		return 1
	}
	c, err := rebuild(path, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "tool-index: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "catalog rebuilt: %d tool(s) at %s\n", len(c.Tools), path)
	return 0
}

func cmdSyncPermissions(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sync-permissions", flag.ContinueOnError)
	fs.SetOutput(stderr)
	settings := fs.String("settings", "", "path to the Claude Code settings.json to update (required)")
	write := fs.Bool("write", false, "apply the changes; without this, entries are only printed")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *settings == "" {
		fmt.Fprintln(stderr, "tool-index: sync-permissions requires --settings PATH")
		return 2
	}
	path := filepath.Clean(*settings)

	dir, err := policy.FindDir()
	if err != nil {
		fmt.Fprintf(stderr, "tool-index: %v\n", err)
		return 1
	}
	set, err := policy.Load(dir)
	if err != nil {
		fmt.Fprintf(stderr, "tool-index: %v\n", err)
		return 1
	}
	existing, err := permissions.ReadAllow(path)
	if err != nil {
		fmt.Fprintf(stderr, "tool-index: %v\n", err)
		return 1
	}
	add := permissions.Missing(set.AutoMode(), existing)
	if len(add) == 0 {
		fmt.Fprintln(stdout, "permissions already up to date")
		return 0
	}
	for _, a := range add {
		fmt.Fprintln(stdout, a)
	}
	if !*write {
		fmt.Fprintf(stdout, "\n%d entr(ies) would be added to %s; re-run with --write to apply\n", len(add), path)
		return 0
	}
	if err := permissions.Apply(path, add); err != nil {
		fmt.Fprintf(stderr, "tool-index: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "\nadded %d entr(ies) to %s (backup at %s.bak)\n", len(add), path, path)
	return 0
}

func writeJSON(stdout, stderr io.Writer, v any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(stderr, "tool-index: %v\n", err)
		return 1
	}
	return 0
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
cd /Users/cding/Workspace/Utils/tool-index && go test ./...
```
Expected: PASS, all packages.

- [ ] **Step 5: Verify end-to-end against the real policy file**

```bash
cd /Users/cding/Workspace/Utils/tool-index
go install ./cmd/...
UTIL_TOOLS_HOME=/Users/cding/Workspace/Utils tool-index refresh
UTIL_TOOLS_HOME=/Users/cding/Workspace/Utils tool-index search registry
```
Expected: `refresh` reports 1 tool; `search` prints `tool-index  Discover and describe the util-tools system commands`.

```bash
UTIL_TOOLS_HOME=/Users/cding/Workspace/Utils tool-index sync-permissions \
  --settings /Users/cding/Workspace/.claude/settings.json
```
Expected: `permissions already up to date` (the shipped policy has `tool-index` in `ask` mode, so nothing is proposed) — and the settings file is untouched. Confirm with `git -C /Users/cding/Workspace diff --stat` equivalent: the file's mtime should be unchanged.

- [ ] **Step 6: Create `tool-index/README.md`**

```markdown
# tool-index

The util-tools registry. It finds and describes the other tools; it cannot run
them.

```
tool-index search <terms...>                 find tools matching keywords
tool-index describe <name>                   print one tool's full descriptor
tool-index refresh                           rebuild the cached catalog
tool-index sync-permissions --settings PATH [--write]
```

Global flags: `--json`, `--describe`.

## How it works

`tools.toml` lists which tools agents may discover. For each exposed tool,
`tool-index` runs `<tool> --describe` and caches the descriptors in
`<os.UserCacheDir()>/util-tools/catalog.json`, so a search does not spawn a
process per tool. `refresh` rebuilds that cache; it runs automatically if the
cache is missing.

A tool that is listed but not installed produces a warning, not a failure —
one broken binary cannot blank the registry.

## Environment

| Variable | Does |
|---|---|
| `UTIL_TOOLS_HOME` | Directory holding `tools.toml` (normally this repo) |
| `UTIL_TOOLS_CONFIG` | Explicit path to a `tools.toml`, overriding the above |
| `UTIL_TOOLS_CATALOG` | Explicit path to the catalog cache |

## sync-permissions

Prints the `Bash(<tool>:*)` allowlist entries implied by `mode = "auto"`
tools. It does **not** modify the settings file unless `--write` is passed,
and it copies the original to `<path>.bak` before writing. Tools with
`expose = "off"` are never granted an entry, whatever their mode.
```

- [ ] **Step 7: Commit**

```bash
cd /Users/cding/Workspace/Utils
jj commit -m "Add tool-index CLI: search, describe, refresh, sync-permissions

sync-permissions prints by default and only writes with --write, since
it edits a large hand-maintained settings file. The catalog rebuilds
itself when the cache is missing.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 8: Workspace wiring

Registers the modules in the workspace `go.work` so tools resolve the local `shared/go` rather than a published pseudo-version, and confirms the mise tasks work across modules.

**Files:**
- Modify: `/Users/cding/Workspace/go.work`

**Interfaces:**
- Consumes: the modules from Tasks 2, 3.
- Produces: a workspace in which `go build ./...` resolves `shared/go` locally from any tool.

- [ ] **Step 1: Add the modules to `go.work`**

`_template` is included so CI and local builds keep it honest.

```
go 1.25.1

use (
	./ChenWeb
	./Kratos/backend
	./ThirdParty/tigerfs
	./Utils/_template
	./Utils/tool-index
	./shared-projects/pdf-proc
	./shared/go
	./tax
)
```

- [ ] **Step 2: Verify the workspace resolves**

```bash
cd /Users/cding/Workspace && go work sync && go build ./Utils/tool-index/...
```
Expected: no output (success).

**Verified behaviour — do not treat as a bug:** `go build ./Utils/...` does NOT
work from the workspace root. Go rejects it with "directory prefix Utils does
not contain modules listed in go.work or their selected dependencies", because
the pattern prefix must itself be a module directory. This is pre-existing and
general — `go build ./ThirdParty/...` fails identically and predates this plan.
Address each module directly, as above.

Separately, `./...` patterns skip `_`-prefixed directories, so no workspace-root
pattern reaches `_template` regardless.
`go.work` still accepts `use ./Utils/_template`, and the module builds and
tests normally from inside its own directory. Confirm it explicitly:

```bash
cd /Users/cding/Workspace/Utils/_template && go test ./...
```
Expected: PASS, with the workspace active (`go env GOWORK` prints the
workspace `go.work` path).

- [ ] **Step 3: Verify the mise tasks work across modules**

```bash
cd /Users/cding/Workspace/Utils
mise test-all
```
Expected: `==> testing _template/` and `==> testing tool-index/`, both with passing tests.

```bash
mise install-all
```
Expected: only `==> installing tool-index/` — `_template` is skipped.

```bash
command -v tool-index
```
Expected: a path under `$(go env GOPATH)/bin`, proving Task 1's PATH fix holds.

- [ ] **Step 4: Confirm no regression in the other workspace modules**

```bash
cd /Users/cding/Workspace && go vet ./shared/go/... 2>&1 | tail -5
```
Expected: no new errors attributable to the `go.work` change.

- [ ] **Step 5: Commit**

The `go.work` file lives in the workspace root, which is not a git repository, so there is nothing to commit for it. Commit only the Utils-side state:

```bash
cd /Users/cding/Workspace/Utils
jj status
```
Expected: clean, or only expected changes. If `jj status` shows changes, commit them:

```bash
jj commit -m "Verify cross-module mise tasks and workspace wiring

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 9: Cross-platform CI

Turns "runs on Windows" from an intention into a verified claim. Development happens on macOS, so this is the only thing actually checking the Linux and Windows rules in `AGENTS.md`.

**Files:**
- Create: `Utils/.github/workflows/ci.yml`

**Interfaces:**
- Consumes: the module layout from Tasks 2, 3.
- Produces: CI enforcement of the cross-platform rules.

- [ ] **Step 1: Create the workflow**

The matrix includes `_template`, so the scaffold cannot rot. The module list is explicit rather than discovered, because a shell glob loop would need different syntax on Windows.

```yaml
name: CI

on:
  push:
  pull_request:
  workflow_dispatch:

jobs:
  test:
    name: ${{ matrix.module }} on ${{ matrix.os }}
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        module: [_template, tool-index]
    defaults:
      run:
        working-directory: ${{ matrix.module }}
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
          # _template has no external dependencies and therefore no go.sum;
          # setup-go fails when cache-dependency-path matches no file. The
          # dependency set is one TOML library, so the cache is worth nothing.
          cache: false

      - name: Verify formatting
        shell: bash
        run: |
          unformatted="$(gofmt -l .)"
          if [ -n "$unformatted" ]; then
            echo "These files are not gofmt'd:"
            echo "$unformatted"
            exit 1
          fi

      - name: Vet
        run: go vet ./...

      - name: Test
        run: go test ./...

      - name: Build
        run: go build ./...
```

- [ ] **Step 2: Verify the workflow locally as far as possible**

CI cannot be fully run locally, but every step it performs can be:

```bash
cd /Users/cding/Workspace/Utils/_template && gofmt -l . && go vet ./... && go test ./... && go build ./...
cd /Users/cding/Workspace/Utils/tool-index && gofmt -l . && go vet ./... && go test ./... && go build ./...
```
Expected: no output from `gofmt -l`, no vet errors, tests pass.

- [ ] **Step 3: Verify the Windows path rules by cross-compiling**

This catches build-level platform mistakes without a Windows machine:

```bash
cd /Users/cding/Workspace/Utils/tool-index && GOOS=windows GOARCH=amd64 go build ./... && GOOS=linux GOARCH=amd64 go build ./...
cd /Users/cding/Workspace/Utils/_template && GOOS=windows GOARCH=amd64 go build ./... && GOOS=linux GOARCH=amd64 go build ./...
```
Expected: success, no output.

- [ ] **Step 4: Commit**

```bash
cd /Users/cding/Workspace/Utils
jj commit -m "Add cross-platform CI matrix

Builds, vets, formats and tests every module on ubuntu, macos and
windows. Development happens on macOS, so this is what actually verifies
the cross-platform rules in AGENTS.md.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 10: The `sys-tools` and `create-sys-tools` skills

Two skills, sourced from `~/Workspace/.agents/skills/` and symlinked into both skill directories per the workspace convention. Neither restates `AGENTS.md`.

**Files:**
- Create: `/Users/cding/Workspace/.agents/skills/sys-tools/SKILL.md`
- Create: `/Users/cding/Workspace/.agents/skills/create-sys-tools/SKILL.md`
- Create: symlinks in `~/.claude/skills/` and `/Users/cding/Workspace/.claude/skills/`

**Interfaces:**
- Consumes: the `tool-index` CLI (Task 7); `AGENTS.md` (Task 1); `_template/` (Task 2).
- Produces: agent-facing discovery and creation workflows.

- [ ] **Step 1: Create the `sys-tools` discovery skill**

The description is deliberately generic: it is the only thing that enters context by default, and it must not name individual tools or the catalog would be loaded wholesale — defeating the purpose.

`/Users/cding/Workspace/.agents/skills/sys-tools/SKILL.md`:

```markdown
---
name: sys-tools
description: Use when a task involves manipulating files, documents, or other repetitive local work that a purpose-built command might already do - searches the workspace's installed system tools before writing a one-off script.
---

# Workspace System Tools

The workspace has a set of installed command-line tools for recurring local
tasks. They are discovered on demand, not listed here — the catalog grows, and
loading all of it into context would waste it.

## Before writing a one-off script

Search for an existing tool first:

```bash
tool-index search <keywords>
```

Use the words a user would actually say: `tool-index search bibtex citations`,
`tool-index search pdf extract`. Nothing found is a normal outcome — proceed
with your own approach.

## Using a tool you found

Get its full interface before calling it:

```bash
tool-index describe <name>
```

This gives arguments, flags, defaults and examples. Then run the tool
directly. Every tool supports `--json`; **use it** — parsing a tool's
human-readable output is a bug waiting to happen.

## Rules

- `tool-index` is a registry, not a dispatcher. It cannot run tools; it only
  finds and describes them. Run the tool yourself.
- A tool may prompt for permission on first use. That is expected; do not try
  to work around it.
- If a tool you expected does not appear, it may exist but be unexposed to
  agents. Tell the user rather than assuming it is absent — they control
  exposure in `Utils/tools.toml`.
- Exit codes: `0` success, `1` tool error, `2` usage error.

To build a new tool rather than find one, use the `create-sys-tools` skill.
```

- [ ] **Step 2: Create the `create-sys-tools` skill**

It is a procedure. The rules live in `AGENTS.md`, and the skill reads that file rather than duplicating it — so the two cannot diverge.

`/Users/cding/Workspace/.agents/skills/create-sys-tools/SKILL.md`:

```markdown
---
name: create-sys-tools
description: Use when creating a new command-line utility for the workspace toolbox - scaffolds a tool in the util-tools repo following its conventions contract, wires it into the workspace, and registers it with the correct default exposure.
---

# Creating a System Tool

Tools live in `/Users/cding/Workspace/Utils` (repo `chendingplano/util-tools`).

**Read `/Users/cding/Workspace/Utils/AGENTS.md` first.** It is the authoritative
conventions contract. This skill is the procedure; it does not restate the
rules, and where the two appear to disagree, `AGENTS.md` wins.

## Procedure

1. **Name the tool.** Lowercase, hyphenated, verb-or-noun, no `util-`/`tool-`
   prefix. Check it does not collide with an existing command:

   ```bash
   command -v <name> || echo "free"
   ```

   Refuse a name that shadows an existing binary and propose alternatives.

2. **Confirm the interface with the user before writing code.** What is the
   input, what is the output, what does `--json` emit, does it mutate files in
   place (if so it needs `--dry-run`). Do not guess.

3. **Scaffold from the template:**

   ```bash
   cd /Users/cding/Workspace/Utils
   cp -R _template <name>
   cd <name>
   ```

4. **Rewrite the identity:**
   - `go.mod` module path → `github.com/chendingplano/util-tools/<name>`
   - `cmd/example-tool/` → `cmd/<name>/`
   - `internal/example/` → a package named for what it does
   - the `descriptor` constant: `name` MUST equal the binary name; `summary`
     one line; `keywords` must include the words a user would actually type
     when searching, or the tool will be undiscoverable
   - `README.md`

5. **Register the module** in `/Users/cding/Workspace/go.work` under `use (`.

6. **Register the policy** in `/Users/cding/Workspace/Utils/tools.toml`:

   ```toml
   [<name>]
   expose = "off"
   mode = "ask"
   ```

   **Always `off`/`ask`.** Exposing a tool to agents is the user's decision,
   made deliberately after the tool has proven itself. Never register a new
   tool as discoverable, and never as `auto`, even if asked to "set it up
   fully" — raise it as a separate question instead.

7. **Write the failing test first.** Table-driven, against the library
   package. Include a CRLF case for any tool that parses text — it is the
   cross-platform rule most often violated.

8. **Implement** the library, then the thin `cmd/`.

9. **Verify:**

   ```bash
   cd /Users/cding/Workspace/Utils/<name>
   gofmt -l . && go vet ./... && go test ./...
   GOOS=windows GOARCH=amd64 go build ./... && GOOS=linux GOARCH=amd64 go build ./...
   ```

   All must pass. The cross-compiles catch platform mistakes without a Windows
   machine.

10. **Install and check the descriptor round-trips:**

    ```bash
    cd /Users/cding/Workspace/Utils && mise install-all
    <name> --describe | python3 -m json.tool
    ```

    Expected: valid JSON whose `name` matches the binary.

11. **Commit with `jj`**, never `git commit`. Do not create branches.

## Afterwards

Tell the user the tool is built and registered as invisible to agents, and ask
whether they want to change `expose`/`mode` in `tools.toml`. If they do, run:

```bash
cd /Users/cding/Workspace/Utils && tool-index refresh
```

so the new tool enters the catalog.
```

- [ ] **Step 3: Create the symlinks**

```bash
ln -s /Users/cding/Workspace/.agents/skills/sys-tools /Users/cding/.claude/skills/sys-tools
ln -s /Users/cding/Workspace/.agents/skills/create-sys-tools /Users/cding/.claude/skills/create-sys-tools
cd /Users/cding/Workspace/.claude/skills
ln -s ../../.agents/skills/sys-tools sys-tools
ln -s ../../.agents/skills/create-sys-tools create-sys-tools
```

- [ ] **Step 4: Verify the symlinks resolve**

```bash
ls -l /Users/cding/.claude/skills/sys-tools /Users/cding/Workspace/.claude/skills/sys-tools
head -4 /Users/cding/Workspace/.claude/skills/create-sys-tools/SKILL.md
```
Expected: both symlinks resolve, and the frontmatter is readable through them.

- [ ] **Step 5: Verify the discovery path works end to end**

```bash
cd /Users/cding/Workspace/Utils
tool-index refresh && tool-index search tools && tool-index describe tool-index
```
Expected: the catalog builds, search finds `tool-index`, and describe prints its arguments and examples — the exact sequence the `sys-tools` skill instructs an agent to perform.

- [ ] **Step 6: Commit**

The skills live in `.agents/skills/`, which is its own git repo per the workspace CLAUDE.md. Check where it is and commit there:

```bash
cd /Users/cding/Workspace/.agents && jj status
```

If it is a `jj` repo, commit:

```bash
cd /Users/cding/Workspace/.agents
jj commit -m "Add sys-tools and create-sys-tools skills

sys-tools keeps a deliberately generic description so the catalog is
discovered on demand rather than loaded into context. create-sys-tools
is a procedure only; AGENTS.md remains the source of truth for the
conventions, and new tools are always registered off/ask.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

If it is not a repo, report that to the user rather than initializing one uninvited.

- [ ] **Step 7: Final verification of the whole plan**

```bash
cd /Users/cding/Workspace/Utils && mise test-all && mise install-all && jj log --no-pager -r 'all()' | head -30
```
Expected: all tests pass, `tool-index` installs, and `jj log` shows a linear history with no divergent or unnamed commits.

---

## Notes for the executor

- **`jj`, never `git commit`.** Raw `git` is for read-only inspection only. After each commit, `jj status` should be clean and `jj log` linear.
- **Do not create `typst-ref`.** It is explicitly out of scope; its requirement document does not exist yet.
- **Do not create branches or bookmarks**, including a `main` bookmark, unless the user asks.
- **Task 1 Step 6 and Step 7 modify things outside the repo** (`~/.zshrc`, `~/Workspace/bin`). Show the user what you found before changing either.
- **Task 7 Step 5 and Task 10 touch `~/Workspace/.claude/settings.json`** only in read or dry-run mode. Never run `sync-permissions --write` against the real settings file as part of this plan.
