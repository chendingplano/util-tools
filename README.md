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
