# netflix-cli

Unofficial, agent-friendly CLI for [netflix.com](https://www.netflix.com), in Go.

It talks to the same endpoints the Netflix web app uses — the account state
embedded in each server-rendered page, and the persisted GraphQL operations the
browser issues for search, title detail and list management — presenting
Chrome's TLS fingerprint (uTLS) so the traffic looks like the web app's.

**The CLI never handles your password.** Sign in to Netflix in your browser as
usual, then import that session:

```sh
netflix login --from-browser chrome
netflix whoami
```

## Install

```sh
go install github.com/seifreed/netflixcli/cmd/netflix@latest
```

or from a clone:

```sh
make build     # ./netflix
make check     # gofmt, vet, tests, build
```

## Read commands

| command | what it does |
| --- | --- |
| `netflix search <query>` | search the catalogue (`--limit N`) |
| `netflix title <id\|url>` | full detail: synopsis, cast, genres, runtime, rating (`--similar`) |
| `netflix seasons <show-id>` | a show's seasons |
| `netflix episodes <show-id>` | a season's episodes (`--season N`, `--all`, `--limit N`) |
| `netflix open <id\|url>` | open the title page, or the player with `--watch` |
| `netflix browse [surface]` | rows of a browse page: `home`, `my-netflix`, `latest`, `games` or a genre id |
| `netflix mylist` | titles saved in this profile's My List |
| `netflix continue` | titles this profile is part-way through |
| `netflix liked` | titles this profile gave a thumbs up |

## Write commands

These change the profile's account state:

| command | what it does |
| --- | --- |
| `netflix mylist add <id\|url>` | save a title to My List |
| `netflix mylist remove <id\|url>` | drop a title from My List |
| `netflix rate <id\|url> up\|down\|love\|none` | set this profile's thumb rating |

## Account

| command | what it does |
| --- | --- |
| `netflix profiles` | list the account's profiles (`*` marks the active one) |
| `netflix profile use <name\|guid>` | re-point the stored session at another profile |
| `netflix history` | this profile's viewing activity, newest first (`--limit N`, `--csv`) |

`--profile <name>` works on **every** command: it acts as that profile for that
one invocation, leaving the stored session where it was. `profile use` is what
changes it for good.

## Session

| command | what it does |
| --- | --- |
| `netflix login --from-browser chrome` | lift cookies from a browser's store (chrome, chromium, firefox, safari, edge, brave) |
| `netflix import-har --file netflix.har` | import a DevTools HAR ("Save all as HAR with sensitive data") |
| `netflix set-cookie '<cookie header>'` | seed a raw Cookie header (`--stdin` supported) |
| `netflix whoami` | show the account the session belongs to |

The session is cached in `~/.netflix/session.json` (mode 0600).
`~/.netflix/config.toml` holds defaults:

```toml
[defaults]
profile = "…"      # profile guid used by default
lang = "es-ES"
```

## How it talks to Netflix

Two paths, each the cheapest one for the job:

**Browse surfaces** (home, My Netflix, a genre) come out of the page itself.
Netflix ships every row it renders as an Apollo cache embedded in the HTML, so
one page fetch yields My List, Continue Watching and the editorial rows without
replaying the page-assembler query. Row titles are localised, so the CLI matches
the personal rows on the feed id Netflix encodes in each row's page actions, not
on their names.

**Search and title detail** are not in the page, so they go to the GraphQL
gateway the web app uses. It sends *persisted* operations — an id, not a query
document — and those ids change with every Netflix build. The CLI therefore
scrapes the id map out of the Akira client bundle once per build and caches it
in `~/.netflix/queries.json`: the first command after a Netflix deploy downloads
that bundle, every later one is a single request.

## Output for agents

Every command takes `--json`, `--jsonl` or `--toon`
([TOON](https://github.com/toon-format/toon-go) is a token-oriented format that
costs fewer tokens than JSON). Data goes to stdout, diagnostics to stderr, so
piping is always safe.

## Environment

| variable | purpose |
| --- | --- |
| `NETFLIX_CONFIG_DIR` | override `~/.netflix` |
| `NETFLIX_BASE_URL` | override the host (debugging proxy, mock) |
| `NETFLIX_CHROME_CDP_URL` | existing Chrome DevTools endpoint for `--browser` |

## Legal

Not affiliated with, endorsed by, or sponsored by Netflix. It automates a
browser session you already own, for your own account. Respect Netflix's Terms
of Use.

## License

MIT — see [LICENSE](LICENSE).
