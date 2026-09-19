# tool-index

The util-tools registry. It finds and describes the other tools; it cannot run
them.

```
tool-index search <terms...>                 find tools matching keywords
tool-index describe <name>                   print one tool's full descriptor
tool-index refresh                           rebuild the cached catalog
tool-index sync-permissions --settings PATH [--write]
```

Global flags: `--json`, `--describe`. Either flag may be given before or after
the subcommand (`tool-index --json search foo` and
`tool-index search foo --json` are equivalent).

## How it works

`tools.toml` lists which tools agents may discover. For each exposed tool,
`tool-index` runs `<tool> --describe` and caches the descriptors in
`<os.UserCacheDir()>/util-tools/catalog.json`, so a search does not spawn a
process per tool. `refresh` rebuilds that cache; it runs automatically if the
cache is missing.

`search` and `describe` re-read `tools.toml` on every invocation and filter
the cached catalog against it, so revoking `expose` takes effect immediately
— you do not need to run `refresh` for a revocation to hide a tool. The
cache still exists to avoid spawning a process per tool; only the (cheap)
policy read happens on every query.

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
and it copies the original to `<path>.bak` before writing — or to
`<path>.bak.<unix-timestamp>` if `<path>.bak` already exists, so a second
sync never destroys the backup of the pre-first-change original. Tools with
`expose = "off"` are never granted an entry, whatever their mode. `tool-index`
itself can never be `mode = "auto"` — see the policy section in the top-level
README — so it can never appear in these entries.

The settings file is rewritten as `map[string]json.RawMessage`, which
marshals with keys in sorted order. The first `--write` against a
hand-maintained file will therefore produce a whole-file key-reordering diff
even though no permissions actually changed relative order; this is
expected and harmless.
