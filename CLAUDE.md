# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`gator` — a CLI RSS feed aggregator in Go, backed by PostgreSQL. Started as the Boot.dev backend course project; everything beyond the core (users/feeds/follows/aggregation) is the maintainer's own extension work — see `docs/roadmap.md` for what's done and what's next.

## Commands

```bash
go build ./...            # build
go vet ./...              # vet
gofmt -l .                # list unformatted files (should be empty)
gofmt -w .                # format
go test ./...             # all tests
go test -run TestParsePubDate ./...   # single test
```

### Database

Schema and queries are **not** hand-written Go — they're generated. After editing anything under `sql/`:

```bash
goose -dir sql/schema postgres "$DB_URL" up   # apply migrations
sqlc generate                                  # regenerate internal/database/
```

Migrations are applied only by the external `goose` CLI against a clone of this repository;
there is no in-binary migrate path yet, so a released archive cannot create its own schema.
A **new migration must be added under `sql/schema`**, never anywhere else.

Never edit `internal/database/*.sql.go` by hand; it is sqlc output (`sqlc.yaml` → `out: internal/database`).

### Running

Requires `~/.gatorconfig.json` with `db_url` (and `current_user_name`, written by `login`/`register`). It has to be created by hand — there is no `gator init`. `internal/config` rewrites it atomically at `0600` (temp file in the same directory, then `os.Rename`), so a crash mid-write cannot truncate it. Local dev Postgres runs on a non-default port; the connection string lives in that config file, not in the repo.

### Releasing

`.goreleaser.yaml` builds `gator` for linux/darwin/windows × amd64/arm64 and stamps the tag into `internal/cli.version` via `-ldflags -X`. Validate without releasing:

```bash
goreleaser check
goreleaser build --snapshot --clean
```

`.github/workflows/ci.yml` gates every push on gofmt, vet, tests and a cross-compile matrix — **keep `gofmt -l .` empty or CI goes red.** There is no release workflow yet; a tagged release means running `goreleaser` by hand.

## Architecture

**Command dispatch.** One table, `allCommands()` in `internal/cli/cli.go`, is the single source of truth: it carries each command's name, args, summary, group and handler, and from it are derived the registry (`command.go`), the interactive picker and `gator help`. A command sets exactly one of `run` (needs config + DB) or `runAuth` (also needs a logged-in user) — `needsLogin()` reads `runAuth != nil`, so the middleware wrapping and what the menu claims can never drift apart. `noDB: true` marks the commands that run before a config exists (`version`, `help`); `cli.Run` only opens config and Postgres when the chosen command actually needs them, so a fresh install can still run those two. `main.go` is now just `cli.Run`. A handler is just `func(*state, command) error` — errors bubble up to `main` which prints and exits 1. Handlers never call `os.Exit` themselves.

**Auth middleware.** Commands needing a logged-in user are wrapped in `middlewareLoggedIn` (`middleware.go`), which converts the signature to `func(*state, command, database.User) error` by looking up `s.Cfg.CurrentUserName`. "Logged in" is purely a username in the config file — there are no passwords or sessions.

**Bare `gator`** opens a command picker (`internal/menu`, Bubble Tea) instead of erroring. It offers only what can actually work: with no reachable database just `version`/`help`, logged out only the guest commands, logged in everything visible (`reset` is `hidden`). The chosen command runs **after** the picker exits, or `tui` and `discover` would nest one Bubble Tea program inside another. A pipe gets the usage text on stderr and exit 1.

**Handlers are split by domain**, all in `package cli`: `users.go`, `feeds.go`, `bookmarks.go`, `categories.go`, `discover.go`, `opml.go`, `stats.go`, `article.go`, `prune.go`, `reset.go`, `service.go`, `tui.go`, `version.go`. Logic shared with the TUI lives in `internal/feeds` (`Add`, `AddMany`, `Follow`, `Scrape`, `ScrapeAll`, `Prune`, `ParsePubDate`) — handlers keep only argument parsing and printing. `feeds.Add` validates a URL by actually fetching it, derives the name from `<title>` when none is given, and treats an already-known URL as "follow it" rather than an error.

**Aggregation loop.** `agg <duration>` ticks on a `time.Ticker`; each tick calls `feeds.ScrapeAll`, which picks up feeds via `GetFeedsToFetch` and fans out one goroutine per feed under a `sync.WaitGroup`. It returns a `[]feeds.Result` and optionally calls an `onResult` callback per finished feed (serialized under a mutex) so the CLI can print progress live while the TUI takes the summary. Duplicate posts are expected — `errors.As` on `*pq.Error` code `23505` skips them. Shutdown is via `signal.NotifyContext`.

**`supervise`** (`service.go`) re-execs the binary as `<self> agg <interval>` in its own process group, restarts it with exponential backoff (1s→60s, reset after 30s of healthy uptime), tees output to stdout and `gator-agg.log`, and forwards SIGINT/SIGTERM to the child. The three things that differ per OS — the signal set, the process group, and how the child is stopped — live behind `shutdownSignals()`, `isolateProcessGroup()` and `terminate()` in `service_unix.go` / `service_windows.go`. **Nothing else in the codebase may import `syscall` directly**; that is what broke the Windows build for weeks.

**Dynamic SQL in sqlc.** `GetPostsForUserFiltered` and `SearchPostsForUser` push optional filtering/sorting into SQL using named params (`@feed_name::text = '' OR ...`, `@feed_id::uuid = '000...'::uuid OR ...`, `NOT @unread_only::bool OR ...`, `CASE WHEN @sort_dir::text = 'asc' THEN ...`) rather than string-building queries in Go. Follow this pattern for new filters.

**`gator reset` (`reset.go`) truncates all six tables**, it does not delete users and let cascades do the rest — after migration 014 that would have left orphan feeds and posts behind, because `feeds.user_id` no longer cascades. `TruncateAll` names every table explicitly and deliberately omits `goose_db_version`, so the schema and migration history survive and the database is usable immediately. Three guards: `localDatabaseName` refuses any `db_url` that is not loopback, the row counts are printed first, and `confirm` requires the database name typed in full rather than `y`. `--yes` skips only the prompt; `--dry-run` skips the delete. `isTerminal` is the `os.ModeCharDevice` check, which counts `/dev/null` as a terminal — the name-match is what actually stops an unattended delete, not that check.

**Deleting a user must not delete anyone else's data.** `feeds.user_id` is nullable and its FK is `ON DELETE SET NULL` (migration 014) — the column means "who first added this feed", not "who owns it". Before that it was `ON DELETE CASCADE`, which chained `users` → `feeds` → `posts` → every other user's `feed_follows`, `bookmarks` and `post_reads`: deleting one user wiped the feeds they had added for everybody. **Never give `feeds.user_id` a cascading delete again, and never add one to `posts`.** The cascades on `feed_follows`, `bookmarks` and `post_reads` are correct and should stay — those rows belong to the user being deleted. `GetFeeds` therefore needs `LEFT JOIN users` with `COALESCE(users.name, '')`, which keeps `user_name` a plain `string` in sqlc; `gator feeds` omits the `Added by:` line when it is empty.

**Per-user post state.** `bookmarks` and `post_reads` both key on `(user_id, post_id)` with `ON CONFLICT DO NOTHING`, same as `feed_follows` — pressing a key twice must never surface a 23505. The TUI mirrors both into `map[uuid.UUID]bool` shared by reference with every `postItem`, so markers (`●` unread, `★` saved) update without rebuilding the list. Mutate those maps, never reassign them.

**TUI** (`internal/tui/`, in progress on branch `DEV-001_TUI`) uses Bubble Tea's Elm architecture — `Init`/`Update`/`View` on a `model`. `internal/cli/tui.go` stays a thin `tui.Run(s.Db, user.ID)` wrapper behind `middlewareLoggedIn`. `docs/tui-plan.md` is the phased plan being followed; it is the source of truth for this work. Two rules that matter: never do DB/HTTP/`exec` work directly inside `Update` (it blocks the event loop — wrap it in a `tea.Cmd`), and always forward `tea.WindowSizeMsg` dimensions to sub-components.

## Code style

The maintainer is actively working toward **idiomatic Go** — prefer the idiomatic form even when a working alternative exists, and point out non-idiomatic patterns in code you touch.

**Do not write comments.** Code that is written by hand carries no comments — not in Go, not in SQL, not in YAML, and not doc comments on exported identifiers either. Explain the reasoning in the commit message or in `docs/`, where it can be read without cluttering the code. Two things are not covered by this rule: files sqlc generates (`internal/database/*.sql.go`), which are never hand-edited anyway, and the directives tools require to work — `-- name: ... :many` for sqlc and `-- +goose Up` / `-- +goose Down` in `sql/schema` — which are instructions to a program, not commentary. Comments already in the tree may stay; this governs what gets added.

Concretely, in this codebase:

- **Type switches over chained type assertions.** In Bubble Tea `Update`, use `switch msg := msg.(type)` with a `case` per message type, not nested `if v, ok := msg.(T); ok`.
- **`errors.Is` / `errors.As` over type assertions on errors** (see `feeds.isDuplicate`).
- **`%w` over `%v` when wrapping errors**, so callers can unwrap.
- **Thread `context.Context`** instead of calling `context.Background()` at every DB call site.
- Keep `gofmt` clean and errors on stderr, output on stdout.

Existing code does not fully follow these yet — improving it as you go is welcome, but keep refactors scoped to what the task touches.

## Notes

- `docs/` is gitignored (along with `*.log`) but present locally; `docs/refactor-plan.md` and `docs/roadmap.md` track known issues and planned features and are worth reading before larger changes.
- The maintainer writes in Serbian; all of `docs/` and the comments still left in the code are in Serbian. Match the language of surrounding text. User-facing CLI output and the README are in English.
- `browse` opens the TUI when stdout is a terminal; `--no-tui` (or a pipe) gives plain output. `--limit`/`--page`/`--feed`/`--sort` apply to plain output only.
- **Both fetch paths bound the response body at 8 MB, and neither truncates silently.** `rss.FetchFeed` reads through `readBody`, which takes `maxBody+1` bytes and returns `ErrBodyTooLarge` if it got more; a silently truncated feed is malformed XML, so a plain `io.LimitReader` would surface as a confusing parse error or, worse, parse with items quietly missing. `article.Fetch` wraps the body in a `boundedReader` that counts bytes and checks `exceeded()` **after** `Extract` returns — the post-check rather than the returned error, because go-readability is not guaranteed to propagate a reader error with `%w`. That one matters more than it looks: `fetchFullText` feeds the result to `article.Improves` and then `SetPostFullText`, so a truncated article would be accepted (it is longer than the feed's teaser) and cached in `posts.full_text` permanently, never refetched. Both limits are generous on purpose — the largest real feed is ~390 KB, the biggest HTML page discovery scans (theverge.com) ~906 KB, and the biggest real article page ~1.8 MB.
- `internal/rss` dispatches on the root element: `<rss>` (RSS 2.0), `<feed>` (Atom) and `<RDF>` (RSS 1.0) are all parsed and converted to the RSS 2.0 shape, so callers only ever see `Channel`/`Item`. Anything else fails with a `*rss.NotAFeedError` naming the root element. A `CharsetReader` backed by `golang.org/x/text` accepts feeds declaring ISO-8859-1 or windows-1252.
- Feed discovery runs on **any** body that is not a recognised feed, including one the XML tokenizer chokes on — YouTube's HTML has unquoted attributes, so `rootElement` fails before it can report `<html>`, and bailing there used to make channel pages unaddable. It also scans the **whole** document rather than stopping at `<body>`: YouTube puts its `<link rel="alternate">` about 750 KB in, below the body. Head links still win because document order is preserved.
- A YouTube channel URL therefore needs no special case: `addfeed` finds `feeds/videos.xml?channel_id=…` on the page. `atomEntry` reads `media:description` (`http://search.yahoo.com/mrss/`) because YouTube leaves `<summary>` and `<content>` empty, so without it every video is a bare title.
- When that root is `<html>`, the same error also carries the feeds the page advertises in `<link rel="alternate">`, read with `golang.org/x/net/html` out of the body already fetched — no second request. `feeds.resolve` takes the first (sites list the main feed before comments feeds) and stores **that** URL, so `addfeed` accepts a bare site address and `export` still writes the feed. Relative hrefs resolve against the URL the request finished on, after redirects.
- `rss.parseFeed` ends with `resolveBody`, which sets each item's `Description` to the longest text the feed offers — `<content:encoded>` (Atom: `<content>`) when present, `<description>` otherwise. Callers never choose. Posts already in the database keep their old short body, because `CreatePost` uses `ON CONFLICT DO NOTHING`.
- `gator stats` (`stats.go`) answers "which of these do I actually read". One query, `GetFeedStatsForUser`, returns volume, reads, bookmarks and the last post/read per followed feed. Two things there are deliberate: the 7-day window arrives as `@since::timestamp` from Go rather than `NOW()` in SQL, matching `GetPostsForUserFiltered`; and `MAX()` is wrapped in `COALESCE(..., '0001-01-01')` because sqlc types a cast aggregate as non-null `time.Time`, so a feed with no posts would panic on a NULL scan. Go reads that zero time as "never".
- `rss.Source` is what is remembered about a feed between fetches: its URL plus the conditional-GET validators. `FetchFeed` takes one and returns the next, and `Source.URL` changes **only** when every hop in the redirect chain was `301`/`308` — a `CheckRedirect` hook watches `req.Response.StatusCode` for that, since `res.Request.URL` alone cannot tell a permanent move from a temporary one. `feeds.Scrape` then writes it back with `SetFeedUrl`, tolerating `23505` because two feeds can redirect onto the same address. Of the maintainer's 49 feeds, 10 redirect permanently.
- Feed health lives on `feeds`: `etag`/`last_modified` (conditional GET) and `last_error`/`failure_count`. `feeds.Scrape` records health **only around `rss.FetchFeed`** — a database write failing is not the feed's fault — and a `304` counts as success. `gator feeds` prints `⚠ failing since N attempt(s)`, the TUI feed pane prefixes the name with `⚠`.
- `internal/opml` reads and writes subscription lists. `Parse` walks arbitrarily nested outlines, treats any outline without an `xmlUrl` as a folder, and keeps the first occurrence of a duplicate URL. `import`/`export` accept `-` for stdin/stdout, and export's summary goes to stderr so the OPML can be piped.
- Folders survive the OPML round trip. They live on `feed_follows.category` (per user — the same feed can sit in different folders for different people): `import` keeps them, `export` groups by them (folders alphabetical, uncategorized last), `discover --add` writes the catalog label, and `gator categorize <url> <folder>` moves one by hand. The TUI feed pane is still a flat list.
- `printPost` prints a bounded preview: `internal/text` strips the HTML and `Truncate` cuts on a word boundary at 400 runes. The full body is only shown in the TUI. `internal/text` is the shared home for both — the TUI detail pane uses `StripHTML` too.
