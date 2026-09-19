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
