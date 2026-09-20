// Command netflix is an unofficial, agent-friendly CLI for netflix.com.
//
// It talks to the same endpoints the Netflix web app uses: the account state
// embedded in each server-rendered page, and the persisted GraphQL operations
// the browser issues for search, title detail and list management. Requests
// carry Chrome's TLS fingerprint (uTLS) and a browser session cookie imported
// with `login --from-browser`, `import-har` or `set-cookie` — the CLI never
// handles a password. Every command supports --json / --toon (data to stdout,
// logs to stderr) for programmatic use.
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Build metadata, injected at release time via -ldflags.
var (
	version = "dev"
	commit  = ""
	date    = ""
)

// Exit codes: 0 success, 1 a command failed, 2 the invocation was wrong.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usageText())
		return exitUsage
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Println(versionString())
		return exitOK
	case "help", "-h", "--help":
		fmt.Fprint(os.Stderr, usageText())
		return exitOK
	}
	cmd, rest, ok := lookup(args)
	if !ok {
		if subs := subcommandsOf(args[0]); len(subs) > 0 {
			fmt.Fprintf(os.Stderr, "usage: netflix %s %s <id|url>\n", args[0], strings.Join(subs, "|"))
			return exitUsage
		}
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		fmt.Fprint(os.Stderr, usageText())
		return exitUsage
	}
	if err := cmd.run(rest); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		var usage usageError
		if errors.As(err, &usage) {
			return exitUsage
		}
		return exitError
	}
	return exitOK
}

func versionString() string {
	if commit == "" {
		return version
	}
	if date != "" {
		return fmt.Sprintf("%s (%s, %s)", version, commit, date)
	}
	return fmt.Sprintf("%s (%s)", version, commit)
}
