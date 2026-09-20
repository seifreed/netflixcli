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
//
// A subcommand is a row like any other, named in full ("mylist add"). Dispatch
// takes the longest matching name, so the parent row and its subcommands live
// side by side without a hand-written switch between them.
type command struct {
	name    string // "search", or "mylist add" for a subcommand
	aliases []string
	group   group
	args    string   // operand syntax shown after the name
	summary string   // one line
	detail  []string // extra help lines, usually flags
	run     func([]string) error
}

// words is how many tokens spell this command's name.
func (c command) words() int { return strings.Count(c.name, " ") + 1 }

// matches reports whether spelling names this command, by name or by alias.
func (c command) matches(spelling string) bool {
	if c.name == spelling {
		return true
	}
	for _, alias := range c.aliases {
		if alias == spelling {
			return true
		}
	}
	return false
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
			name: "top", group: groupRead,
			summary: "Netflix's top 10 series and films in this country",
			detail:  []string{"--limit N   cap titles per list"},
			run:     cmdTop,
		},
		{
			name: "browse", group: groupRead, args: "[surface]",
			summary: "rows of a browse page (home by default)",
			detail: []string{
				"surface: home | my-netflix | <genre-id> (see genres)",
				"--all       every row, not just the eight in the page",
				"--limit N   cap titles per row",
			},
			run: cmdBrowse,
		},
		{
			name: "mylist", aliases: []string{"my-list"}, group: groupRead,
			summary: "titles saved in this profile's My List",
			run:     cmdFeed("mylist", client.FeedMyList),
		},
		{
			name: "continue", aliases: []string{"continue-watching"}, group: groupRead,
			summary: "titles this profile is part-way through",
			run:     cmdFeed("continue", client.FeedContinueWatching),
		},
		{
			name: "liked", group: groupRead,
			summary: "titles this profile gave a thumbs up",
			run:     cmdFeed("liked", client.FeedLiked),
		},
		{
			name: "reminders", group: groupRead,
			summary: "titles this profile is waiting for",
			detail:  []string{"(all four take --limit N)"},
			run:     cmdFeed("reminders", client.FeedReminders),
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
			run:     cmdMyListAdd,
		},
		{
			name: "mylist remove", aliases: []string{"mylist rm"},
			group: groupWrite, args: "<id|url>",
			summary: "drop a title from My List",
			run:     cmdMyListRemove,
		},
		{
			name: "rate", group: groupWrite, args: "<id|url> <rating>",
			summary: "thumb rating: up | down | love | none",
			run:     cmdRate,
		},
		{
			name: "continue remove", aliases: []string{"continue rm"},
			group: groupWrite, args: "<id>",
			summary: "drop a title from Continue Watching",
			detail:  []string{"(the viewing history entry stays; not undoable)"},
			run:     cmdContinueRemove,
		},
		{
			name: "remind add", group: groupWrite, args: "<id|url>",
			summary: "release reminder for a title that is not out yet",
			detail: []string{
				"(Netflix files an already-available title in My List",
				"instead, and the reply says so)",
			},
			run: cmdRemindAdd,
		},
		{
			name: "remind remove", aliases: []string{"remind rm"},
			group: groupWrite, args: "<id|url>",
			summary: "drop a title's release reminder",
			run:     cmdRemindRemove,
		},
		{
			name: "profiles", aliases: []string{"profile", "profile list"},
			group:   groupAccount,
			summary: "list the account's profiles (* marks the active one)",
			run:     cmdProfiles,
		},
		{
			name: "profile use", group: groupAccount, args: "<name|guid>",
			summary: "re-point the stored session at another profile",
			run:     cmdProfileUse,
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
	}
}

// maxCommandWords bounds how many leading arguments can spell one command.
// "mylist add" is two; no name or alias in the table is longer.
const maxCommandWords = 2

// lookup resolves the command spelled by the leading arguments and returns the
// operands left after it. The longest spelling wins, so `mylist add 123`
// reaches the subcommand while a bare `mylist` reaches the parent row.
func lookup(args []string) (cmd command, rest []string, ok bool) {
	table := commands()
	for words := maxCommandWords; words >= 1; words-- {
		if len(args) < words {
			continue
		}
		spelling := strings.Join(args[:words], " ")
		for _, c := range table {
			if c.matches(spelling) {
				return c, args[words:], true
			}
		}
	}
	return command{}, nil, false
}

// helpRows are the table entries the help lists — every one of them, since each
// row is dispatchable on its own.
func helpRows() []command { return commands() }

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
  NETFLIX_BASE_URL         override the page host (debugging proxy, mock)
  NETFLIX_GRAPHQL_URL      override the GraphQL gateway (debugging proxy, mock)
  NETFLIX_CONFIG_DIR       override ~/.netflix
  NETFLIX_CHROME_CDP_URL   existing Chrome DevTools endpoint

  version | help
`

// subcommandsOf lists the subcommands spelled under name, so a parent that only
// exists as a prefix — `netflix remind` — can say what it is missing instead of
// reporting itself as unknown. It reads the table, so it cannot fall behind it.
func subcommandsOf(name string) []string {
	var subs []string
	for _, c := range commands() {
		if parent, sub, ok := strings.Cut(c.name, " "); ok && parent == name {
			subs = append(subs, sub)
		}
	}
	sort.Strings(subs)
	return subs
}
