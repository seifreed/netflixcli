<p align="center">
  <img src="https://img.shields.io/badge/netflix--cli-Netflix%20CLI-E50914?style=for-the-badge" alt="netflix-cli">
</p>

<h1 align="center">netflix-cli</h1>

<p align="center">
  <strong>Unofficial, agent-friendly command-line client for netflix.com</strong>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/seifreed/netflixcli"><img src="https://img.shields.io/badge/pkg.go.dev-reference-007d9c?style=flat-square&logo=go&logoColor=white" alt="Go Reference"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/seifreed/netflixcli?style=flat-square&logo=go&logoColor=white" alt="Go Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License"></a>
  <a href="https://github.com/seifreed/netflixcli/actions"><img src="https://img.shields.io/github/actions/workflow/status/seifreed/netflixcli/ci.yml?style=flat-square&logo=github&label=CI" alt="CI Status"></a>
  <a href="https://github.com/seifreed/netflixcli/security/code-scanning"><img src="https://img.shields.io/badge/code%20scanning-SARIF%20enabled-brightgreen?style=flat-square" alt="SARIF"></a>
</p>

<p align="center">
  <a href="https://github.com/seifreed/netflixcli/stargazers"><img src="https://img.shields.io/github/stars/seifreed/netflixcli?style=flat-square" alt="GitHub Stars"></a>
  <a href="https://github.com/seifreed/netflixcli/issues"><img src="https://img.shields.io/github/issues/seifreed/netflixcli?style=flat-square" alt="GitHub Issues"></a>
  <a href="https://buymeacoffee.com/seifreed"><img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-support-yellow?style=flat-square&logo=buy-me-a-coffee&logoColor=white" alt="Buy Me a Coffee"></a>
</p>

---

## Overview

**netflix-cli** searches the Netflix catalogue, reads title detail, and manages
the signed-in profile's lists from the terminal. It talks to the same endpoints
the Netflix web app uses and presents Chrome's TLS fingerprint, so the traffic
looks like the browser's.

**It never handles your password.** Sign in to Netflix in your browser as usual;
the CLI lifts that session out of the browser's cookie store.

```sh
netflix login --from-browser chrome
netflix whoami
```

### Key Features

| Feature | Description |
|---------|-------------|
| **No password, ever** | Imports the session from a browser you already signed in to (Chrome, Firefox, Safari, Edge, Brave) |
| **Catalogue search** | Pages past the first 48 results, up to whatever Netflix has |
| **Title detail** | Synopsis, cast, directors, writers, genres, runtime, certification, similar titles |
| **Seasons and episodes** | Per-season episode lists with synopsis, runtime and your resume point |
| **Top 10** | Netflix's own ranking for your country — the rows the page does not render |
| **Personal rows** | My List, Continue Watching, liked titles, reminders |
| **Writes** | Add to and remove from My List, thumb ratings, reminders, drop from Continue Watching |
| **Profiles** | List them, act as one for a single command, or switch the stored session |
| **Viewing history** | Full activity export with dates normalised to ISO 8601 |
| **Agent-friendly output** | `--json`, `--jsonl` and `--toon`; data on stdout, logs on stderr |

### Supported Outputs

```text
Structured      JSON, JSONL, TOON (token-oriented, fewer tokens than JSON)
Human           Aligned tables and title cards
Exports         Viewing history as CSV
Agent skill     .claude/skills/netflix-browse for Claude Code
```

---

## Installation

### From a Release

Every tag publishes prebuilt archives for Linux, macOS and Windows on amd64 and
arm64 (Windows on amd64 only), with a `checksums.txt` beside them. Download the
one for your platform from
[Releases](https://github.com/seifreed/netflixcli/releases), verify it, and put
the binary on your `PATH`:

```bash
shasum -a 256 -c checksums.txt --ignore-missing
tar xzf netflix_*_darwin_arm64.tar.gz
```

### From Source (Recommended)

```bash
go install github.com/seifreed/netflixcli/cmd/netflix@latest
```

### From a Clone

```bash
git clone https://github.com/seifreed/netflixcli.git
cd netflixcli
make build          # ./netflix
```

### Verify the Build

```bash
make check          # gofmt, vet, tests, build
make gate           # the full quality and security gates
```

---

## Quick Start

```bash
# Import the session from a browser you are signed in to
netflix login --from-browser chrome

# Who is this?
netflix whoami

# What is most watched in this country right now?
netflix top

# Is it on Netflix?
netflix search "breaking bad" --limit 5

# Tell me about it
netflix title 70143836
```

---

## Usage

### Read Commands

| Command | Description |
|---------|-------------|
| `netflix search <query>` | Search the catalogue. `--limit N` pages past the first 48 |
| `netflix top` | Netflix's top 10 series and films in this country |
| `netflix title <id\|url>` | Full detail. `--similar` resolves the related ids |
| `netflix seasons <show-id>` | A show's seasons |
| `netflix episodes <show-id>` | Episodes. `--season N`, `--all`, `--limit N` |
| `netflix genres [filter]` | Genres this region offers, with the ids `browse` takes |
| `netflix browse [surface]` | Rows of a browse page: `home`, `my-netflix` or a genre id. `--all` fetches every row |
| `netflix mylist` | Every title saved in this profile's My List (`--limit N` for a sample) |
| `netflix continue` | Titles this profile is part-way through |
| `netflix liked` | Titles this profile gave a thumbs up |
| `netflix reminders` | Titles this profile is waiting for |
| `netflix open <id>` | Open in the system browser. `--watch` goes straight to the player |

### Write Commands

These change the profile's account state.

| Command | Description |
|---------|-------------|
| `netflix mylist add <id>` | Save a title to My List |
| `netflix mylist remove <id>` | Drop a title from My List |
| `netflix rate <id> <rating>` | Thumb rating: `up`, `down`, `love` or `none` |
| `netflix continue remove <id>` | Drop a title from Continue Watching (not undoable) |
| `netflix remind add\|remove <id>` | Release reminder for a title that is not out yet |

### Account and Session

| Command | Description |
|---------|-------------|
| `netflix profiles` | List the account's profiles; `*` marks the active one |
| `netflix profile use <name>` | Re-point the stored session at another profile |
| `netflix history` | Viewing activity, newest first. `--limit N`, `--csv` |
| `netflix login --from-browser b` | Lift cookies from a browser's store |
| `netflix import-har --file f` | Import a DevTools HAR ("Save all as HAR with sensitive data") |
| `netflix set-cookie '<cookie>'` | Seed a raw Cookie header. `--stdin` keeps it out of shell history |
| `netflix whoami` | Show the account and the profile the session acts as |

### Common Flags

| Option | Description |
|--------|-------------|
| `--json` | Emit raw JSON (data → stdout, logs → stderr) |
| `--jsonl` | One JSON object per line |
| `--toon` | [TOON](https://github.com/toon-format/toon-go) — same fields as JSON, far fewer tokens |
| `--profile <name\|guid>` | Act as another profile for this one invocation |
| `--lang es-ES` | UI language for titles and labels |
| `--browser` | Fetch through an already-running Chrome via CDP |

Flags may appear anywhere after the command.

---

## How It Talks to Netflix

Two paths, each the cheapest one for the job:

**Browse surfaces** come out of the page itself. Netflix server-renders every
row it shows as an Apollo cache embedded in the HTML, so one page fetch yields
My List, Continue Watching and the editorial rows. Row titles are localised, so
the CLI matches the personal rows on the feed id Netflix encodes in each row's
page actions, not on their names.

**Search, title detail, episodes and every write** go to the GraphQL gateway as
*persisted* operations — an id, not a query document. Those ids change with
every Netflix build, so the CLI scrapes them out of the client bundle once per
build and caches the map in `~/.netflix/queries.json`. The first command after a
Netflix deploy downloads that bundle; every later one is a single request.

---

## Quality and Security Gates

Both gates are pinned to exact tool versions, so a gate means the same thing on
every machine and in CI.

### Quality Gate — `make quality`

| Check | Tool | What it catches |
|-------|------|-----------------|
| Formatting | `gofmt` | Unformatted source |
| Correctness | `go vet` | Suspicious constructs the compiler allows |
| Lint | `golangci-lint` | staticcheck, revive, gocritic, errcheck, ineffassign, unused, gosec, govet |
| Dependency drift | `go mod tidy -diff` | `go.mod`/`go.sum` that are not what `tidy` would write |
| Concurrency | `go test -race` | Data races |
| Coverage floor | `coverage-gate.sh` | Total coverage below the floor (a ratchet, never lowered) |

### Security Gate — `make security`

| Check | Tool | What it catches |
|-------|------|-----------------|
| Reachable vulnerabilities | `govulncheck` | CVEs on code paths the binary actually reaches |
| Dependency vulnerabilities | `osv-scanner` | CVEs anywhere in the dependency graph, reachable or not |
| Secret scanning | `gitleaks` | A session cookie or token committed to history |
| Supply chain | `go mod verify` | Module checksums that disagree with `go.sum` |

`make gate` runs both plus the build — the same thing CI runs. CI also uploads
the static-analysis findings as SARIF 2.1.0 to GitHub Code Scanning.

---

## Agent Use

The repository ships a [Claude Code](https://claude.com/claude-code) skill at
`.claude/skills/netflix-browse`. It teaches an agent which command answers which
question, to chain by title id rather than by localised name, what each command
costs in requests, which commands write to the account, and what the CLI cannot
do — so it neither guesses nor invents.

Pair it with `--toon` to keep the token cost of results down.

---

## Configuration

`~/.netflix/config.toml` holds defaults:

```toml
[defaults]
profile = "…"      # profile guid or name used by default
lang = "es-ES"
```

| Path | Contents |
|------|----------|
| `~/.netflix/session.json` | Cached Cookie header, mode 0600. Never print it |
| `~/.netflix/queries.json` | Persisted GraphQL query ids for the current build; safe to delete |
| `~/.netflix/config.toml` | Defaults above, plus an optional `[auth] cookie` |

| Variable | Purpose |
|----------|---------|
| `NETFLIX_CONFIG_DIR` | Override `~/.netflix` |
| `NETFLIX_BASE_URL` | Override the page host (debugging proxy, mock) |
| `NETFLIX_GRAPHQL_URL` | Override the GraphQL gateway (debugging proxy, mock) |
| `NETFLIX_CHROME_CDP_URL` | DevTools endpoint of an already-running Chrome, for `--browser` |

---

## Requirements

- Go 1.26.6+ (see [go.mod](go.mod)) — to build from source; a release archive needs nothing
- A browser you are signed in to Netflix with, for the initial session import

### Supported Platforms

Built and tested on every push, on all three:

| Platform | Built | Tested in CI |
|---|---|---|
| Linux (amd64, arm64) | yes | yes, with the race detector |
| macOS (amd64, arm64) | yes | yes, with the race detector |
| Windows (amd64) | yes | yes |

The race detector needs a C toolchain, which is certain on the Linux and macOS
images; a data race is not platform-specific, and this CLI's only concurrency is
a single `sync.Once`, so the Windows job runs the same tests without it.

---

## Contributing

Contributions are welcome.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Run `make gate` and make sure it is green
5. Open a Pull Request

### Cutting a Release

A release is made by pushing a tag, and only by pushing a tag:

```bash
git tag -a v1.2.0 -m "v1.2.0"
git push origin v1.2.0
```

The tag is the version. `release.yml` first runs the very same gates a push to
`main` runs — the three-platform build and test matrix, the quality gate and the
security gate — so a tag cannot publish something `main` would have rejected.
Only then does GoReleaser cross-compile the archives, write `checksums.txt` and
attach them to the GitHub release for that tag.

The tag is stamped into the binary, so an installed copy can always say where it
came from:

```console
$ netflix version
v1.2.0 (a1b2c3d, 2026-09-20T19:06:07Z)
```

---

## Legal

Not affiliated with, endorsed by, or sponsored by Netflix. It automates a
browser session you already own, for your own account, and reads only what that
account can already see. Respect Netflix's Terms of Use.

---

## Support the Project

If this project is useful in your workflows, you can support development:

<a href="https://buymeacoffee.com/seifreed" target="_blank">
  <img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" height="50">
</a>

---

## License

This project is licensed under the MIT license. See [LICENSE](LICENSE).

**Attribution**
- Author: **Marc Rivero López** | [@seifreed](https://github.com/seifreed)
- Repository: [github.com/seifreed/netflixcli](https://github.com/seifreed/netflixcli)

---

<p align="center">
  <sub>Built for terminal-first and agent-driven Netflix workflows</sub>
</p>
