---
name: netflix-browse
description: >-
  Search Netflix, read title detail, and manage the signed-in profile's My List, ratings and viewing
  history through the local netflix CLI. Use it to find what is on Netflix in this account's region,
  check whether something is available, inspect a title's cast, genres or runtime, list what the
  profile is part-way through, or export viewing activity. Netflix only — not Prime Video, Disney+
  or other services. It never reads or asks for a password.
---

# Netflix browse

Use the `netflix` binary in `/Users/seifreed/tools/personal/netflixcli` to answer questions about
what is on Netflix **for this account, in this account's region**, and to manage the active
profile's lists.

## Non-negotiable facts

- The CLI never handles a password. It carries a session lifted from a browser the user already
  signed in to.
- Never print, store, log, commit or paste cookie values, and never put one in a command line that
  ends up in shell history — `set-cookie --stdin` exists for that.
- The catalogue is regional and personalised. Results describe *this* account, not Netflix at large;
  say so when it matters.
- `mylist add`, `mylist remove` and `rate` change the user's account. Run them only when the user
  asked for that change. Everything else is read-only.
- Do not invent commands. If something is not in the reference below, it does not exist.

## Start here

```sh
netflix whoami            # is the session live, and whose is it?
netflix login --from-browser chrome   # only if whoami fails
```

If `whoami` reports the session is not signed in, the fix is always to re-import from a browser
where the user is signed in — never to ask for credentials.

## Picking a command

| The user wants | Command |
|---|---|
| "is X on Netflix?", "find me …" | `netflix search "<query>" --limit 10` |
| detail: plot, cast, year, runtime, rating | `netflix title <id\|url>` |
| "something like X" | `netflix title <id> --similar` |
| "how many seasons / what are the episodes?" | `netflix seasons <id>` then `netflix episodes <id> --season N` |
| what genres exist / "algo de terror" | `netflix genres [filter]`, then `netflix browse <genre-id>` |
| what is on the home page / a genre | `netflix browse [home\|<genre-id>]` |
| "what's on my list?" | `netflix mylist` |
| "what am I in the middle of?" | `netflix continue` |
| "what have I watched?" | `netflix history --limit 50` |
| "what am I waiting for?" | `netflix reminders` |
| save / unsave a title | `netflix mylist add\|remove <id>` |
| thumbs up / down | `netflix rate <id> up\|down\|love\|none` |
| open it to watch | `netflix open <id> --watch` |

Ids come from `search`; every command that takes an id also takes a `netflix.com/title/<id>` URL.

## Working with the output

Add `--toon` when feeding results back into your own reasoning: it carries the same fields as
`--json` in far fewer tokens. Use `--json` when the user wants the data, and `--jsonl` to stream
rows into other tools. Data goes to stdout and diagnostics to stderr, so pipes stay clean.

Chain by id, not by name: `search` → take the `id` → `title <id>` for detail, or `mylist add <id>`.
Titles are localised and ambiguous; ids are not.

## Profiles

`netflix profiles` lists them and marks the active one. Two different things:

- `--profile "<name>"` on any command acts as that profile for that one call, **without switching**.
- `netflix profile use "<name>"` re-points the stored session and affects every later command.
  Prefer the first unless the user asked to switch.

A PIN-locked profile cannot be entered from the CLI; say so and let the user unlock it in the
browser.

## When something fails

| Symptom | What it means |
|---|---|
| `no Netflix session` | nothing imported yet — run `login --from-browser`. |
| session "is not signed in" | the cookie expired — re-run `login --from-browser`. |
| `does not expose the GraphQL operation` | Netflix shipped a new build; the query map refreshes itself on the next run. |
| a title is missing | it is not in this region's catalogue — that is an answer, not an error. |

See `references/cli-reference.md` for the full flag list.
