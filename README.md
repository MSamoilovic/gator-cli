# Gator

A CLI RSS feed aggregator built as part of the [Boot.dev](https://www.boot.dev) backend course. Follow feeds, aggregate posts, and manage subscriptions from the terminal.

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
gator addfeed "Feed Name" https://example.com/feed.xml   # add a feed and follow it
gator feeds                                               # list all feeds
gator follow https://example.com/feed.xml                # follow an existing feed
gator unfollow https://example.com/feed.xml              # unfollow a feed
gator following                                          # list feeds you follow
```

### Aggregation

```bash
gator agg       # fetch and print the latest posts from your feeds
```

### Admin

```bash
gator reset     # delete all users (use with caution)
```
