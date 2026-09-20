package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/seifreed/netflixcli/internal/client"
)

// reorderArgs exists so flags may follow positional arguments, which the stdlib
// parser refuses.
func TestReorderArgsAllowsTrailingFlags(t *testing.T) {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	limit := fs.Int("limit", 0, "")
	asJSON := fs.Bool("json", false, "")
	parseFlags(fs, []string{"breaking", "bad", "--limit", "5", "--json"})
	if *limit != 5 || !*asJSON {
		t.Fatalf("limit=%d json=%v, want 5 and true", *limit, *asJSON)
	}
	if got := strings.Join(fs.Args(), " "); got != "breaking bad" {
		t.Errorf("positional = %q, want %q", got, "breaking bad")
	}
}

func TestReorderArgsStopsAtDoubleDash(t *testing.T) {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "")
	parseFlags(fs, []string{"--", "--json", "literal"})
	if *asJSON {
		t.Error("a flag after -- must be treated as a positional argument")
	}
	if got := strings.Join(fs.Args(), " "); got != "--json literal" {
		t.Errorf("positional = %q, want the arguments after -- untouched", got)
	}
}

// A negative number is a value, not a flag.
func TestReorderArgsKeepsNegativeNumbers(t *testing.T) {
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	parseFlags(fs, []string{"-5", "-1.5"})
	if got := strings.Join(fs.Args(), " "); got != "-5 -1.5" {
		t.Errorf("positional = %q, want the negative numbers preserved", got)
	}
}

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

func TestCSVField(t *testing.T) {
	for in, want := range map[string]string{
		"plain":         "plain",
		"with, comma":   `"with, comma"`,
		`with "quotes"`: `"with ""quotes"""`,
		"with\nnewline": "\"with\nnewline\"",
	} {
		if got := csvField(in); got != want {
			t.Errorf("csvField(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "third"); got != "third" {
		t.Errorf("firstNonEmpty = %q, want third", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Errorf("firstNonEmpty = %q, want empty", got)
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	if code := run([]string{"nonsense"}); code != 2 {
		t.Errorf("exit code = %d, want 2 for an unknown command", code)
	}
	if code := run(nil); code != 2 {
		t.Errorf("exit code = %d, want 2 with no arguments", code)
	}
	if code := run([]string{"version"}); code != 0 {
		t.Errorf("exit code = %d, want 0 for version", code)
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

func TestOperand(t *testing.T) {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	parseFlags(fs, []string{"breaking", "bad"})
	got, err := operand(fs, "netflix search <query>")
	if err != nil {
		t.Fatalf("operand: %v", err)
	}
	if got != "breaking bad" {
		t.Errorf("operand = %q, want the arguments joined", got)
	}

	empty := flag.NewFlagSet("search", flag.ContinueOnError)
	parseFlags(empty, nil)
	if _, err := operand(empty, "netflix search <query>"); err == nil {
		t.Fatal("want a usage error when the operand is missing")
	} else if !strings.Contains(err.Error(), "usage: netflix search <query>") {
		t.Errorf("error = %q, want it to spell the usage", err)
	}
}

func TestTitleOperand(t *testing.T) {
	fs := flag.NewFlagSet("title", flag.ContinueOnError)
	parseFlags(fs, []string{"https://www.netflix.com/title/80100172"})
	id, err := titleOperand(fs, "netflix title <id|url>")
	if err != nil {
		t.Fatalf("titleOperand: %v", err)
	}
	if id != 80100172 {
		t.Errorf("id = %d, want the id parsed out of the URL", id)
	}

	bad := flag.NewFlagSet("title", flag.ContinueOnError)
	parseFlags(bad, []string{"not-an-id"})
	if _, err := titleOperand(bad, "netflix title <id|url>"); err == nil {
		t.Fatal("want an error for an operand that is not a title id")
	}
}
