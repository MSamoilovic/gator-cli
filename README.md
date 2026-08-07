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

### Register and log in

```bash
gator register <username>   # create a new user and log in
gator login <username>      # log in as an existing user
gator users                 # list all users
```

### Managing feeds

```bash
gator addfeed https://example.com/feed.xml                # add a feed and follow it (name taken from the feed)
gator addfeed "Feed Name" https://example.com/feed.xml   # ...or name it yourself
gator feeds                                               # list all feeds
gator follow https://example.com/feed.xml                # follow an existing feed
gator unfollow https://example.com/feed.xml              # unfollow a feed
gator following                                          # list feeds you follow
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

#### TUI keys

| Key | Action |
|---|---|
| `↑`/`↓`, `k`/`j` | Move (loads more posts at the bottom) |
| `⏎` | Read post (marks it read) / select feed |
| `u` | Toggle read / unread (`●` = unread) |
| `A` | Mark everything in the list as read |
| `U` | Show only unread posts |
| `o` | Open in browser |
| `b` | Bookmark / unbookmark (`★`) |
| `B` | Show only bookmarked posts |
| `S` | Toggle sort: newest / oldest first |
| `r` | Reload the current view |
| `R` | Fetch all feeds now, then reload |
| `s` | Search all posts in the database |
| `/` | Filter the loaded list by title |
| `tab` | Switch between the feed and post panes |
| `a` | Add a feed by URL (feed pane) |
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
