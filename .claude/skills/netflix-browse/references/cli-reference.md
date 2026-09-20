# `netflix` CLI reference

Data goes to stdout; logs and errors go to stderr. Exit codes: `0` success, `1` runtime error
(expired session, title not in the region), `2` usage or unknown command.

Common flags, valid anywhere after the command: `--json`, `--jsonl`, `--toon`, `--lang es-ES|en`,
`--profile <name|guid>`, `--browser`, `--browser-endpoint URL`.

`--profile` acts as that profile for that one invocation; the stored session is
unchanged. It costs one extra request, so omit it when the active profile is
already the right one.

## Session

| Command | Purpose |
|---|---|
| `netflix login --from-browser chrome` | Import Netflix cookies from a browser and probe the session. Browsers: `chrome`, `chromium`, `firefox`, `safari`, `edge`, `brave`; empty reads any. |
| `netflix import-har --file FILE\|-` | Import a DevTools HAR ("Save all as HAR with sensitive data"). |
| `netflix set-cookie COOKIE` | Seed a raw Cookie header. `--stdin` keeps it out of shell history. |
| `netflix whoami` | Show the account the session belongs to. |

## Read

| Command | Purpose |
|---|---|
| `netflix search QUERY [--limit N]` | Catalogue search. Default page is 48 titles. |
| `netflix title ID\|URL [--similar]` | Synopsis, cast, directors, writers, genres, mood tags, runtime, certification, playback badges, watch status, thumb rating, My List membership. `--similar` resolves the related ids to names in one batched call. |
| `netflix seasons SHOW-ID\|URL` | A show's seasons: number, title, episode count and the season id the next command needs. Errors clearly for a movie. |
| `netflix episodes SHOW-ID\|URL [--season N] [--all] [--limit N]` | Episodes with number, title, synopsis, runtime, and a resume marker where the profile left off. Defaults to the first season. |
| `netflix browse [SURFACE] [--limit N]` | Rows of a browse page. Surfaces: `home` (default), `my-netflix`, `latest`, `games`, or a genre id such as `83`. `--limit` caps titles per row. |
| `netflix mylist [--limit N]` | Titles in this profile's My List. |
| `netflix continue [--limit N]` | Titles this profile is part-way through. |
| `netflix liked [--limit N]` | Titles this profile gave a thumbs up. |
| `netflix history [--limit N] [--csv]` | Viewing activity, newest first, dates as `YYYY-MM-DD`. |
| `netflix open ID\|URL [--watch]` | Open the title page, or the player with `--watch`, in the system browser. |

## Write

These change account state. Run them only on an explicit request.

| Command | Purpose |
|---|---|
| `netflix mylist add ID\|URL` | Save a title to My List. |
| `netflix mylist remove ID\|URL` | Drop a title from My List. |
| `netflix rate ID\|URL up\|down\|love\|none` | Set this profile's thumb rating. |

## Account

| Command | Purpose |
|---|---|
| `netflix profiles` | List profiles; `*` marks the active one. `--json` includes `guid`, `isKids`, `isPinLocked`. |
| `netflix profile use NAME\|GUID` | Re-point the stored session at another profile. Refuses PIN-locked profiles. |

## Environment

| Variable | Purpose |
|---|---|
| `NETFLIX_CONFIG_DIR` | Override `~/.netflix` (session cache, query map, config.toml). |
| `NETFLIX_BASE_URL` | Override the host — for a debugging proxy or a mock. |
| `NETFLIX_CHROME_CDP_URL` | DevTools endpoint of an already-running Chrome, for `--browser`. |

## Files

| Path | Contents |
|---|---|
| `~/.netflix/session.json` | Cached Cookie header, mode 0600. Never print it. |
| `~/.netflix/queries.json` | Persisted GraphQL query ids for the current Netflix build; safe to delete. |
| `~/.netflix/config.toml` | `[defaults] profile`, `lang`; `[auth] cookie`. |
