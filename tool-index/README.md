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
