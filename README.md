# Gator

A CLI RSS feed aggregator. Follow feeds, aggregate posts, and manage subscriptions from the terminal.

> **About this project**
> Gator started as the guided [Boot.dev](https://www.boot.dev) backend course project. The core (users, feeds, follows, aggregation loop) follows the course.
> Beyond that, I've been extending it with my own features that go **outside the course scope** — filtering/sorting & pagination for `browse`, full-text `search`, `bookmark`s, a concurrent aggregator, and a crash-restarting `supervise` service manager. Those are marked **(extra)** below.

## Prerequisites

- **Go** 1.22+
- **PostgreSQL** 15+

## Installation

```bash
go install github.com/MSamoilovic/gator-cli@latest
```

## Setup

### 1. Start PostgreSQL

```bash
sudo service postgresql start       # Linux / WSL
brew services start postgresql@15   # macOS
```

### 2. Create the database

```bash
sudo -u postgres psql               # Linux
psql postgres                       # macOS
```

```sql
CREATE DATABASE gator;
\c gator
ALTER USER postgres PASSWORD 'postgres';
```

### 3. Run database migrations

```bash
goose -dir sql/schema postgres "postgres://<user>:<password>@localhost:<port>/gator" up
```

### 4. Create the config file

Create `~/.gatorconfig.json` with your database connection string:

```json
{
  "db_url": "postgres://postgres:postgres@localhost:5432/gator?sslmode=disable"
}
```

## Usage

Run `gator` with no arguments and it offers the commands you can actually run —
greeting you by name when you are logged in, and showing only `register` /
`login` when you are not. Arrow keys move, `/` filters, `⏎` runs; commands
that take arguments ask for them first.

```bash
gator          # pick a command interactively
gator help     # ...or print the same list as plain text
```

Piping or redirecting (`gator | less`) prints the list and exits non-zero
instead, so a script that forgets the command still fails.

### Register and log in

```bash
gator register <username>   # create a new user and log in
gator login <username>      # log in as an existing user
gator users                 # list all users
```

### Finding feeds to follow (extra)

Don't know any RSS URLs? The binary ships with a curated catalog:

```bash
gator discover                      # interactive picker: space to pick, ⏎ to follow
gator discover --no-tui             # list the categories instead (also what a pipe gets)
gator discover tech                 # show the feeds in one category (✓ = already followed)
gator discover --add tech,sport     # add and follow every feed in those categories
```

Feeds are fetched in parallel and validated before they are stored, so a dead
URL in one category does not stop the rest. Running it twice is safe — a feed
you already follow is reported as *already known*, not as an error.

### Moving in and out (extra)

Subscriptions travel as OPML, the format every reader speaks:

```bash
gator import feedly.opml     # add and follow everything in the file
curl -s $URL | gator import - # ...or straight from a pipe
gator export feeds.opml      # write your subscriptions out
gator export > feeds.opml    # same, on stdout
```

Nested folders are flattened and a feed listed twice is imported once. OPML
carries subscriptions only — not read state, bookmarks or posts.

### Adding a feed you cannot name

Nobody remembers RSS addresses, so `addfeed` takes the site instead. If what you
give it turns out to be a web page, gator reads the `<link rel="alternate">`
tags the page publishes and follows the feed it finds there:

```
$ gator addfeed https://jvns.ca
Added feed "Julia Evans" (https://jvns.ca/atom.xml)
```

When a page advertises several — WordPress publishes a comments feed next to the
articles one, some sites offer both RSS and Atom — the first is taken, which is
the one sites put first. The stored address is the feed's, not the page's, so
`gator export` and later fetches use the right URL. The same applies to `a` in
the TUI feed pane.

### Managing feeds

```bash
gator addfeed https://example.com                         # paste the site; gator finds its feed
gator addfeed https://example.com/feed.xml                # add a feed and follow it (name taken from the feed)
gator addfeed "Feed Name" https://example.com/feed.xml   # ...or name it yourself
gator feeds                                               # list all feeds (⚠ marks ones that fail to fetch)
gator follow https://example.com/feed.xml                # follow an existing feed
gator unfollow https://example.com/feed.xml              # unfollow a feed
gator following                                          # list feeds you follow, grouped by folder
gator categorize <feed_url> <folder>                     # put a feed in a folder (empty folder = root)
```

### Aggregation

```bash
gator agg 1m                 # fetch feeds in a loop every 1m (1s, 5m, 1h, ...); Ctrl+C to stop
gator supervise 1m           # (extra) keep agg running, auto-restart on crash, log to gator-agg.log
```

### Reading posts (extra)

```bash
gator tui                                        # interactive reader (feed panel, search, bookmarks)
gator browse                                     # same TUI, when stdout is a terminal
gator browse --no-tui                            # plain output instead (default 2 posts)
gator browse --no-tui --limit 10 --page 2        # pagination
gator browse --no-tui --feed boot --sort asc     # filter by feed name, sort by publish date
gator search golang --limit 5                    # full-text search across post titles/descriptions
```

`browse` opens the TUI only on an interactive terminal — piping or redirecting
(`gator browse | less`) falls back to plain output, so scripts keep working.
The `--limit`, `--page`, `--feed` and `--sort` flags apply to plain output only.

The feed pane groups by folder, in the same order as `gator following` and
`gator export` — folders alphabetical, uncategorized last. `⏎` on a `▾` header
folds it away; the header keeps showing the folder's total unread count, and a
`⚠` if something inside it is failing to fetch. Folded folders are remembered
between runs. If you have not put anything in a folder yet, the pane stays the
flat list it always was.

The TUI remembers the selected feed, sort order, unread filter and time range in
`~/.gator-state.json`, so it reopens where you left off. Delete that file to start
fresh; a missing or unreadable one is not an error.

#### TUI keys

| Key | Action |
|---|---|
| `↑`/`↓`, `k`/`j` | Move (loads more posts at the bottom) |
| `⏎` | Read post (marks it read) / select feed / fold a folder |
| `u` | Toggle read / unread (`●` = unread) |
| `A` | Mark everything in the list as read |
| `U` | Show only unread posts |
| `t` | Cycle time range: all → 24h → 7d → 30d |
| `o` | Open in browser |
| `y` | Copy the post URL to the clipboard |
| `n` / `p` | Next / previous post while reading |
| `b` | Bookmark / unbookmark (`★`) |
| `B` | Show only bookmarked posts |
| `S` | Toggle sort: newest / oldest first |
| `r` | Reload the current view |
| `R` | Fetch all feeds now, then reload |
| `s` | Search all posts in the database |
| `/` | Filter the loaded list by title |
| `tab` | Switch between the feed and post panes |
| `a` | Add a feed by URL (feed pane) |
| `c` | Browse the catalog by interest (feed pane); `space` picks, `⏎` follows |
| `d` | Unfollow the selected feed (feed pane) |
| `esc` | Back from a post, or out of search results |
| `?` | Expand the help |
| `q` / `ctrl+c` | Quit |

### Bookmarks (extra)

```bash
gator bookmark <post_url>     # save a post
gator bookmarks               # list saved posts (newest first)
gator unbookmark <post_url>   # remove a saved post
```

### Admin

```bash
gator reset     # delete all users (use with caution)
```

`reset` is deliberately left out of the interactive menu — it is still there
when you type it, just not something to land on by accident.
