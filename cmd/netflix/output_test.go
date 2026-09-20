package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/seifreed/netflixcli/internal/client"
)

// A structured flag must suppress the human view entirely, or --json output
// would be polluted with prose.
func TestOutputPicksTheStructuredFormat(t *testing.T) {
	titles := []client.Title{{ID: 1, Title: "A"}, {ID: 2, Title: "B"}}
	for name, tc := range map[string]struct {
		cf   *common
		want string
	}{
		"json":  {&common{jsonOut: true}, `"title": "A"`},
		"jsonl": {&common{jsonl: true}, "\n"},
		"toon":  {&common{toon: true}, "id"},
	} {
		humanRan := false
		out := captureStdout(t, func() {
			if err := output(tc.cf, titles, func() { humanRan = true }); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		})
		if humanRan {
			t.Errorf("%s: the human view ran despite a structured flag", name)
		}
		if !strings.Contains(out, tc.want) {
			t.Errorf("%s output %q does not contain %q", name, out, tc.want)
		}
	}
}

func TestOutputFallsBackToTheHumanView(t *testing.T) {
	humanRan := false
	out := captureStdout(t, func() {
		if err := output(&common{}, []client.Title{{ID: 1}}, func() { humanRan = true }); err != nil {
			t.Fatalf("output: %v", err)
		}
	})
	if !humanRan {
		t.Error("want the human view to run when no structured flag is set")
	}
	if out != "" {
		t.Errorf("output itself wrote %q to stdout, want nothing", out)
	}
}

// --jsonl must emit one object per line, not the array.
func TestEmitJSONLSplitsArrays(t *testing.T) {
	out := captureStdout(t, func() {
		if err := emitJSONL([]client.Title{{ID: 1}, {ID: 2}}); err != nil {
			t.Fatalf("emitJSONL: %v", err)
		}
	})
	if lines := strings.Count(strings.TrimSpace(out), "\n") + 1; lines != 2 {
		t.Errorf("got %d lines, want one per element: %q", lines, out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	stdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = stdout }()
	fn()
	w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	return buf.String()
}
