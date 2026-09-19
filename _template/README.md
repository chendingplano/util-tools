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
