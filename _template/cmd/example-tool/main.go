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
