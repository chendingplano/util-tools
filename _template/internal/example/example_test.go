package example

import (
	"os"
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

// TestCountFromTestdata reads a golden fixture rather than an inline string,
// demonstrating the testdata/ directory the repo conventions contract
// requires every tool to have.
func TestCountFromTestdata(t *testing.T) {
	f, err := os.Open("../../testdata/sample.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	got, err := Count(f)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	want := Result{Lines: 2, Words: 5}
	if got != want {
		t.Errorf("Count(testdata/sample.txt) = %+v, want %+v", got, want)
	}
}
