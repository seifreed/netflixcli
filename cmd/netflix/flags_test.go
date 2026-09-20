package main

import (
	"flag"
	"strings"
	"testing"
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

// A CDP endpoint the CLI will not talk to is a mistake in the invocation, and
// has to be reported as one. It used to be wrapped in a fetcher that failed on
// first use, so it surfaced as a page-load error one call later.
func TestNewClientRejectsARemoteBrowserEndpointUpFront(t *testing.T) {
	t.Setenv("NETFLIX_CONFIG_DIR", t.TempDir())
	cl, err := newClient(&common{browserEndpoint: "http://198.51.100.7:9222"})
	if err == nil {
		t.Fatal("want an error for a non-loopback CDP endpoint")
	}
	if cl != nil {
		t.Error("a client was returned alongside the error")
	}
	if !strings.Contains(err.Error(), "loopback") {
		t.Errorf("error %q does not say what is wrong with the endpoint", err)
	}
}
