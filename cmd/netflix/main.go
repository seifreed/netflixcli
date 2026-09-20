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

	"github.com/seifreed/netflixcli/internal/client"
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
	case "genres":
		err = cmdGenres(args[1:])
	case "browse":
		err = cmdBrowse(args[1:])
	case "mylist", "my-list":
		err = runMyList(args[1:])
	case "continue", "continue-watching":
		err = runContinue(args[1:])
	case "reminders":
		err = cmdFeed(client.FeedReminders)(args[1:])
	case "remind":
		err = cmdRemind(args[1:])
	case "liked":
		err = cmdFeed(client.FeedLiked)(args[1:])
	case "profiles":
		err = cmdProfiles(args[1:])
	case "profile":
		err = cmdProfile(args[1:])
	case "seasons":
		err = cmdSeasons(args[1:])
	case "episodes":
		err = cmdEpisodes(args[1:])
	case "open":
		err = cmdOpen(args[1:])
	case "rate":
		err = cmdRate(args[1:])
	case "history":
		err = cmdHistory(args[1:])
	case "title":
		err = cmdTitle(args[1:])
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

// runContinue dispatches `continue` between listing and dropping a title.
func runContinue(args []string) error {
	if len(args) > 0 && (args[0] == "remove" || args[0] == "rm") {
		return cmdContinueRemove(args[1:])
	}
	return cmdFeed(client.FeedContinueWatching)(args)
}

// runMyList dispatches `mylist` between listing and the two writes.
func runMyList(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "add":
			return cmdMyListAdd(args[1:])
		case "remove", "rm":
			return cmdMyListRemove(args[1:])
		}
	}
	return cmdFeed(client.FeedMyList)(args)
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
                           --limit N   how many titles (pages past the first 48)
  genres [filter]          genres this region offers, with the ids browse takes
  browse [surface]         rows of a browse page (home by default)
                           surface: home | my-netflix | <genre-id> (see genres)
                           --limit N   cap titles per row
  mylist                   titles saved in this profile's My List
  continue                 titles this profile is part-way through
  liked                    titles this profile gave a thumbs up
  reminders                titles this profile is waiting for
                           (all four take --limit N)
  title <id|url>           full detail for one title (synopsis, cast, genres)
                           --similar   also resolve the similar-title ids
  seasons <show-id|url>    a show's seasons
  episodes <show-id|url>   a season's episodes (the first unless --season N)
                           --all       every season
                           --limit N   cap episodes per season
  open <id|url>            open a title in the system browser
                           --watch     open the player instead of the page

WRITE COMMANDS (they change this profile's account state):
  mylist add <id|url>      save a title to My List
  mylist remove <id|url>   drop a title from My List
  rate <id|url> <rating>   thumb rating: up | down | love | none
  continue remove <id>     drop a title from Continue Watching
                           (the viewing history entry stays)
  remind add|remove <id>   release reminder for a title that is not out yet
                           (Netflix files an already-available title in My List
                           instead, and the reply says so)

ACCOUNT:
  profiles                 list the account's profiles (* marks the active one)
  profile use <name|guid>  re-point the stored session at another profile
  history                  this profile's viewing activity, newest first
                           --limit N   cap entries
                           --csv       emit CSV instead of a table

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
