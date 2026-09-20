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
```

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
