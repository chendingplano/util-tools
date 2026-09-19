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
