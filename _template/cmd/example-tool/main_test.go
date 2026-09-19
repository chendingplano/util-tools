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
