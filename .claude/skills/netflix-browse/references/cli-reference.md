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
| `netflix top [--limit N]` | Netflix's top 10 series and films for this country, numbered. They are not in the page — the command pages through the rows to reach them, so it costs about four requests. |
| `netflix genres [FILTER]` | The genres this region offers with the ids `browse` takes. Netflix never shows these ids in the UI, so this is the only way to find one. |
| `netflix browse [SURFACE] [--limit N]` | Rows of a browse page. Surfaces: `home` (default), `my-netflix`, or a genre id such as `8711`. `--limit` caps titles per row. `--all` fetches every row (about 47 on the home page) instead of the eight the page renders, at roughly four requests. Its rows carry the same `feed` markers and keep empty personal rows, so it is a superset of the default. A personal row that exists but is empty is reported as empty, not omitted. |
| `netflix mylist [--limit N]` | Every title in this profile's My List — the whole list, which can be hundreds. `--limit N` for a sample. |
| `netflix continue [--limit N]` | Every title this profile is part-way through. |
| `netflix liked [--limit N]` | Every title this profile gave a thumbs up. |
| `netflix reminders [--limit N]` | Every title this profile is waiting for. |
| `netflix history [--limit N] [--csv]` | Viewing activity, newest first, dates as `YYYY-MM-DD`. |
| `netflix open ID\|URL [--watch]` | Open the title page, or the player with `--watch`, in the system browser. |

## Write

These change account state. Run them only on an explicit request.

| Command | Purpose |
|---|---|
| `netflix mylist add ID\|URL` | Save a title to My List. |
| `netflix mylist remove ID\|URL` | Drop a title from My List. |
| `netflix rate ID\|URL up\|down\|love\|none` | Set this profile's thumb rating. |
| `netflix continue remove ID\|URL` | Drop a title from Continue Watching. The viewing-history entry stays. Not reversible from the CLI — ask before running it. |
| `netflix remind add\|remove ID\|URL` | Release reminder for a title that is not out yet. Netflix files an **already-available** title in My List instead, so the reply reports the reminder and My List flags as they came back rather than claiming a reminder was set. |

## Account

| Command | Purpose |
|---|---|
| `netflix profiles` | List profiles; `*` marks the active one. `--json` includes `guid`, `isKids`, `isPinLocked`. |
| `netflix profile use NAME\|GUID` | Re-point the stored session at another profile. Refuses PIN-locked profiles. |

## Environment

| Variable | Purpose |
|---|---|
| `NETFLIX_CONFIG_DIR` | Override `~/.netflix` (session cache, query map, config.toml). |
| `NETFLIX_BASE_URL` | Override the page host — for a debugging proxy or a mock. |
| `NETFLIX_GRAPHQL_URL` | Override the GraphQL gateway — same purpose. |
| `NETFLIX_CHROME_CDP_URL` | DevTools endpoint of an already-running Chrome, for `--browser`. |

## Files

| Path | Contents |
|---|---|
| `~/.netflix/session.json` | Cached Cookie header, mode 0600. Never print it. |
| `~/.netflix/queries.json` | Persisted GraphQL query ids for the current Netflix build; safe to delete. |
| `~/.netflix/config.toml` | `[defaults] profile`, `lang`; `[auth] cookie`. |
