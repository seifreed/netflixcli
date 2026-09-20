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

// A command that found nothing must answer with an empty list, not null: a
// caller looping over the result should get no rows rather than a type error.
// `search` returned a nil slice and so answered `null`, while `reminders` on an
// empty list answered `[]`.
func TestEmptyResultsAreAnEmptyList(t *testing.T) {
	var noTitles []client.Title
	for _, format := range []struct {
		name string
		cf   *common
		want string
	}{
		{"--json", &common{jsonOut: true}, "[]\n"},
		{"--jsonl", &common{jsonl: true}, ""},
		{"--toon", &common{toon: true}, "[0]:\n"},
	} {
		out := captureStdout(t, func() {
			if err := output(format.cf, noTitles, func() {}); err != nil {
				t.Errorf("%s: %v", format.name, err)
			}
		})
		if out != format.want {
			t.Errorf("%s of no results = %q, want %q", format.name, out, format.want)
		}
	}
}

// A result that is not a list is untouched.
func TestNonListResultsPassThrough(t *testing.T) {
	out := captureStdout(t, func() {
		if err := output(&common{jsonOut: true}, map[string]any{"removed": true}, func() {}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, `"removed": true`) {
		t.Errorf("output = %q", out)
	}
}

// A command whose probe failed must not put a row on stdout. `whoami` with no
// session printed " ()" there, ahead of the error on stderr.
func TestPrintUserSaysNothingWithoutAnAccount(t *testing.T) {
	if out := captureStdout(t, func() { printUser(client.UserInfo{}) }); out != "" {
		t.Errorf("printUser of an empty account wrote %q to stdout", out)
	}
	out := captureStdout(t, func() {
		printUser(client.UserInfo{Name: "Ada", MembershipStatus: "CURRENT_MEMBER"})
	})
	if !strings.Contains(out, "Ada (CURRENT_MEMBER)") {
		t.Errorf("printUser = %q", out)
	}
}
