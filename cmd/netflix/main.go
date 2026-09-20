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
	"fmt"
	"os"

	"github.com/seifreed/netflixcli/internal/config"
)

// Build metadata, injected at release time via -ldflags.
var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) < 1 {
		usage()
		return 2
	}
	var err error
	switch args[0] {
	case "search":
		err = cmdSearch(args[1:])
	case "login":
		err = cmdLogin(args[1:])
	case "import-har":
		err = cmdImportHar(args[1:])
	case "set-cookie":
		err = cmdSetCookie(args[1:])
	case "whoami":
		err = cmdWhoami(args[1:])
	case "version", "--version", "-v":
		fmt.Println(versionString())
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		usage()
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
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

func configDirLabel() string { return config.Dir() }

func usage() {
	fmt.Fprint(os.Stderr, `netflix — unofficial CLI for netflix.com

USAGE:
  netflix <command> [flags]

READ COMMANDS:
  search <query>           search the catalogue
                           --limit N   cap titles returned (default 48)

SESSION (bring the session from a browser you are already signed in to):
  login --from-browser b   lift cookies from a browser's store — easiest
                           (chrome|chromium|firefox|safari|edge|brave; empty = any)
  import-har --file f      import a DevTools HAR ("Save all as HAR with sensitive data")
  set-cookie '<cookie>'    seed a raw Cookie header manually (--stdin supported)
  whoami                   show the account the current session belongs to

COMMON FLAGS (may go anywhere after the command):
  --lang es-ES             UI language for titles and labels
  --profile <guid|name>    act as a specific profile
  --json                   emit raw JSON (data→stdout, logs→stderr)
  --jsonl                  emit one JSON object per line
  --toon                   emit TOON instead of JSON (fewer tokens; for LLM/agents)
  --browser                use the existing Chrome via CDP; never launch another one
  --browser-endpoint URL   local CDP endpoint (default: 127.0.0.1:9222)

ENV:
  NETFLIX_BASE_URL         override the host (debugging proxy, mock)
  NETFLIX_CONFIG_DIR       override ~/.netflix
  NETFLIX_CHROME_CDP_URL   existing Chrome DevTools endpoint

  version | help
`)
}
