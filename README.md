# scrobbles-cli

A fast, modular command-line tool written in Go to sync, normalize,
enrich, and analyze your Last.fm listening history in your own Turso
(libSQL) or SQLite database.

## Features

- **Incremental & Range Sync (`sync`):** Fetches new scrobbles from
  Last.fm starting from your latest stored timestamp, or backfills
  custom date ranges (`--from`, `--to`).
- **Track Alias Normalization (`alias`):** Maps alternate track titles
  (translations, remasters, regional variants) to canonical titles so
  plays, durations, and rankings merge cleanly across all commands.
- **Relational Entity Resolution & Duration Sync (`resolve`):** Links
  raw scrobble logs to normalized `artists` and `tracks` tables in
  transactional batches and enriches tracks with actual durations
  (`duration_ms`) from Last.fm.
- **Comprehensive Listening Analytics (`stats`, `top`, `latest`,
  `list`, `info`):**
    - Total listening time (using actual track durations with
      automatic fallback), active days, daily averages, peak listening
      day/hour, and repeat ratio.
    - Top tracks, artists, and albums over any date window.
    - Combined global Last.fm metadata (tags, listeners, wiki summary)
      paired with personal listening history and library share ratio.
- **Lazy Resource Initialization:** Commands that do not require
  database access (such as `alias`, `version`, and `help`) run without
  opening a remote Turso connection.

## Architecture

`scrobbles-cli` follows a layered Repository Pattern with lazy
dependency injection:

```text
cmd/ (Cobra CLI + Lazy App Container)
 └──► internal/service/ (Business logic, alias merging, Last.fm API orchestration)
       └──► internal/repository/ (Unified persistence interfaces & *sql.DB / JSON implementations)
             └──► Turso (libSQL) / Local SQLite & aliases.json
```

## Installation

### Build from Source

```bash
git clone https://github.com/mbagusaditya/scrobbles-cli.git
cd scrobbles-cli
go build -o scrobbles .
```

Optionally move the binary into your `$PATH`:

```bash
mv scrobbles ~/.local/bin/
```

## Configuration

Configure your Last.fm credentials and Turso/SQLite database connection
via environment variables or your configuration file
(`internal/config`):

```env
LASTFM_API_KEY=your_lastfm_api_key
LASTFM_USERNAME=your_lastfm_username

# Turso (libSQL) or local SQLite file URL
TURSO_DATABASE_URL=libsql://your-database.turso.io
TURSO_AUTH_TOKEN=your_turso_auth_token
```

## Usage

### 1. Sync Scrobbles from Last.fm

```bash
# Incremental sync from your latest recorded scrobble
scrobbles sync

# Sync a specific date range (YYYY-MM-DD)
scrobbles sync --from 2026-11-01 --to 2026-11-07
```

### 2. Resolve Entities & Sync Track Durations

```bash
# Link raw scrobbles to normalized artists & tracks tables
scrobbles resolve

# Fetch missing track durations from Last.fm
scrobbles resolve durations
```

### 3. Manage Track Aliases

```bash
# Add a canonical title mapping
scrobbles alias add --artist "MyGO!!!!!" --from "Sunday" --to "騒混出"

# List configured track aliases
scrobbles alias list

# Remove an alias rule by index
scrobbles alias remove 1
```

### 4. Inspect Listening Statistics & Rankings

```bash
# Overall listening summary (total duration, active days, peak day/hour, repeat ratio)
scrobbles stats
scrobbles stats --from 2026-01-01 --to 2026-12-31

# Top tracks, artists, or albums
scrobbles top tracks --limit 15
scrobbles top artists --from 2026-11-01
scrobbles top albums --limit 10

# Most recently played unique tracks, artists, or albums
scrobbles latest tracks
scrobbles latest artists
scrobbles latest albums
```

### 5. Browse Logs, Library & Entity Details

```bash
# Browse raw scrobble logs with filters
scrobbles list --artist "MyGO!!!!!" --limit 30

# Browse unique catalog items (A-Z)
scrobbles list tracks
scrobbles list artists
scrobbles list albums

# View combined Last.fm metadata + personal play statistics
scrobbles info track "騒混出" --artist "MyGO!!!!!"
scrobbles info artist "MyGO!!!!!"
scrobbles info album "迷跡波" --artist "MyGO!!!!!"
```

## License

MIT
