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
