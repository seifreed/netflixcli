package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/seifreed/netflixcli/internal/client"
)

// The command table is the single source of truth for what the CLI does: `run`
// dispatches from it and `usage` renders from it. Help that is written out by
// hand drifts from the dispatch — this CLI once documented two browse surfaces
// that had been removed — and a table cannot drift from itself.
type command struct {
	name    string
	aliases []string
	group   group
	args    string   // operand syntax shown after the name
	summary string   // one line
	detail  []string // extra help lines, usually flags
	run     func([]string) error
}

// group orders the sections of the help text.
type group int

const (
	groupRead group = iota
	groupWrite
	groupAccount
	groupSession
)

var groupTitles = map[group]string{
	groupRead:    "READ COMMANDS:",
	groupWrite:   "WRITE COMMANDS (they change this profile's account state):",
	groupAccount: "ACCOUNT:",
	groupSession: "SESSION (bring the session from a browser you are already signed in to):",
}

func commands() []command {
	return []command{
		{
			name: "search", group: groupRead, args: "<query>",
			summary: "search the catalogue",
			detail:  []string{"--limit N   how many titles (pages past the first 48)"},
			run:     cmdSearch,
		},
		{
			name: "genres", group: groupRead, args: "[filter]",
			summary: "genres this region offers, with the ids browse takes",
			run:     cmdGenres,
		},
		{
			name: "browse", group: groupRead, args: "[surface]",
			summary: "rows of a browse page (home by default)",
			detail: []string{
				"surface: home | my-netflix | <genre-id> (see genres)",
				"--limit N   cap titles per row",
			},
			run: cmdBrowse,
		},
		{
			name: "mylist", aliases: []string{"my-list"}, group: groupRead,
			summary: "titles saved in this profile's My List",
			run:     runMyList,
		},
		{
			name: "continue", aliases: []string{"continue-watching"}, group: groupRead,
			summary: "titles this profile is part-way through",
			run:     runContinue,
		},
		{
			name: "liked", group: groupRead,
			summary: "titles this profile gave a thumbs up",
			run:     cmdFeed(client.FeedLiked),
		},
		{
			name: "reminders", group: groupRead,
			summary: "titles this profile is waiting for",
			detail:  []string{"(all four take --limit N)"},
			run:     cmdFeed(client.FeedReminders),
		},
		{
			name: "title", group: groupRead, args: "<id|url>",
			summary: "full detail for one title (synopsis, cast, genres)",
			detail:  []string{"--similar   also resolve the similar-title ids"},
			run:     cmdTitle,
		},
		{
			name: "seasons", group: groupRead, args: "<show-id|url>",
			summary: "a show's seasons",
			run:     cmdSeasons,
		},
		{
			name: "episodes", group: groupRead, args: "<show-id|url>",
			summary: "a season's episodes (the first unless --season N)",
			detail: []string{
				"--all       every season",
				"--limit N   cap episodes per season",
			},
			run: cmdEpisodes,
		},
		{
			name: "open", group: groupRead, args: "<id|url>",
			summary: "open a title in the system browser",
			detail:  []string{"--watch     open the player instead of the page"},
			run:     cmdOpen,
		},
		{
			name: "mylist add", group: groupWrite, args: "<id|url>",
			summary: "save a title to My List",
		},
		{
			name: "mylist remove", group: groupWrite, args: "<id|url>",
			summary: "drop a title from My List",
		},
		{
			name: "rate", group: groupWrite, args: "<id|url> <rating>",
			summary: "thumb rating: up | down | love | none",
			run:     cmdRate,
		},
		{
			name: "continue remove", group: groupWrite, args: "<id>",
			summary: "drop a title from Continue Watching",
			detail:  []string{"(the viewing history entry stays; not undoable)"},
		},
		{
			name: "remind", group: groupWrite, args: "add|remove <id>",
			summary: "release reminder for a title that is not out yet",
			detail: []string{
				"(Netflix files an already-available title in My List",
				"instead, and the reply says so)",
			},
			run: cmdRemind,
		},
		{
			name: "profiles", group: groupAccount,
			summary: "list the account's profiles (* marks the active one)",
			run:     cmdProfiles,
		},
		{
			name: "profile use", group: groupAccount, args: "<name|guid>",
			summary: "re-point the stored session at another profile",
		},
		{
			name: "history", group: groupAccount,
			summary: "this profile's viewing activity, newest first",
			detail: []string{
				"--limit N   cap entries",
				"--csv       emit CSV instead of a table",
			},
			run: cmdHistory,
		},
		{
			name: "login", group: groupSession, args: "--from-browser b",
			summary: "lift cookies from a browser's store — easiest",
			detail:  []string{"(chrome|chromium|firefox|safari|edge|brave; empty = any)"},
			run:     cmdLogin,
		},
		{
			name: "import-har", group: groupSession, args: "--file f",
			summary: `import a DevTools HAR ("Save all as HAR with sensitive data")`,
			run:     cmdImportHar,
		},
		{
			name: "set-cookie", group: groupSession, args: "'<cookie>'",
			summary: "seed a raw Cookie header manually (--stdin supported)",
			run:     cmdSetCookie,
		},
		{
			name: "whoami", group: groupSession,
			summary: "show the account the current session belongs to",
			run:     cmdWhoami,
		},
		// `profile` dispatches its own subcommands; the table documents them
		// individually above, so this entry is hidden from the help.
		{name: "profile", run: cmdProfile},
	}
}

// lookup resolves a command name or alias. Entries with no run func are help-only
// rows documenting a subcommand, and are never dispatched to.
func lookup(name string) (command, bool) {
	for _, c := range commands() {
		if c.run == nil {
			continue
		}
		if c.name == name {
			return c, true
		}
		for _, alias := range c.aliases {
			if alias == name {
				return c, true
			}
		}
	}
	return command{}, false
}

// helpRows are the table entries the help lists, in table order. The bare
// `profile` dispatcher is hidden: its subcommands are documented on their own.
func helpRows() []command {
	var rows []command
	for _, c := range commands() {
		if c.name == "profile" {
			continue
		}
		rows = append(rows, c)
	}
	return rows
}

const (
	helpIndent    = "  "
	helpNameWidth = 24 // column the summaries line up in
)

// usageText renders the help from the command table. It is a pure function so
// the layout can be asserted without capturing output.
func usageText() string {
	var b strings.Builder
	b.WriteString("netflix — unofficial CLI for netflix.com\n\nUSAGE:\n  netflix <command> [flags]\n")
	rows := helpRows()
	for _, g := range groupsInOrder() {
		fmt.Fprintf(&b, "\n%s\n", groupTitles[g])
		for _, c := range rows {
			if c.group == g {
				writeCommand(&b, c)
			}
		}
	}
	b.WriteString(commonHelp)
	return b.String()
}

func groupsInOrder() []group {
	seen := map[group]bool{}
	var groups []group
	for _, c := range helpRows() {
		if !seen[c.group] {
			seen[c.group] = true
			groups = append(groups, c.group)
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i] < groups[j] })
	return groups
}

func writeCommand(b *strings.Builder, c command) {
	fmt.Fprintf(b, "%s%-*s %s\n", helpIndent, helpNameWidth, invocation(c), c.summary)
	for _, line := range c.detail {
		fmt.Fprintf(b, "%s%-*s %s\n", helpIndent, helpNameWidth, "", line)
	}
}

// invocation is how a command is spelled in the help: its name plus operands.
func invocation(c command) string {
	if c.args == "" {
		return c.name
	}
	return c.name + " " + c.args
}

// commonHelp covers what is true of every command, so it is not repeated per row.
const commonHelp = `
COMMON FLAGS (may go anywhere after the command):
  --lang es-ES             UI language for titles and labels
  --profile <guid|name>    act as a specific profile for this one invocation
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
`

// commandNames lists every dispatchable name and alias, for error messages and
// tests.
func commandNames() []string {
	var names []string
	for _, c := range commands() {
		if c.run == nil {
			continue
		}
		names = append(names, c.name)
		names = append(names, c.aliases...)
	}
	sort.Strings(names)
	return names
}
