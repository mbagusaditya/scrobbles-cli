# Scrobbles CLI

A high-performance CLI tool written in Go to ingest, synchronize, normalize, and inspect personal Last.fm scrobble history into a Turso (libSQL) database.

Designed to share a unified database schema with an existing web application (built with Drizzle ORM), serving as a decoupled background ingestion, metadata enrichment, and analytics engine.

---

## Features

- **Robust Ingestion (`sync`)**: Fetches scrobbles with cursor pagination, 500ms API rate throttling, and exponential backoff retry.
- **Idempotent Ingestion**: Leverages SQLite unique compound indexes (`raw_artist`, `raw_title`, `played_at`) with `INSERT OR IGNORE` to prevent duplicates.
- **Entity & Duration Enrichment (`resolve`)**: Links raw scrobble logs into normalized master `artists` and `tracks` tables via batch transactions, and enriches `duration_ms` from Last.fm `track.getInfo` (`--durations`).
- **Read-Time Track Alias Mapping (`alias`)**: Merges duplicate track variants (e.g., Kanji vs. Romaji titles) dynamically in-memory via a local XDG `aliases.json` file without mutating raw database records.
- **Accurate Listening Analytics (`stats`)**: Calculates actual listening time using resolved track durations (with transparent fallback estimation), inclusive calendar active days, peak hours, and repeat ratios.
- **Rankings, Library & Deep Inspection (`top`, `list`, `info`)**: Explore top tracks/artists/albums, browse paginated master libraries, and inspect combined Last.fm metadata alongside personal play history.
- **XDG Base Directory Support & Zero CGO**: Automatically resolves global configuration from `~/.config/scrobbles/` and compiles cleanly with `tursodatabase/libsql-client-go`.

---

## Architecture Overview

Employs a strict 4-layer architecture:

```text
cmd/        → CLI orchestration (Cobra), flag parsing, ANSI terminal output
internal/
├── service/ → Core business logic, alias resolution, batch orchestration, SQL execution
├── model/   → Pure domain entities (Artist, Track, Scrobble, Alias, Stats, Top, Info)
├── lastfm/  → HTTP client, URL construction, rate limiting, retry backoff
├── db/      → Turso connection lifecycle management
└── config/  → Environment & alias configuration resolution (CWD & XDG Base Directory)

```

---

## Getting Started

### Prerequisites

- Go 1.25 or higher
- A Turso database instance
- Last.fm API credentials

### Installation

Clone the repository and build the binary:

```bash
git clone [https://github.com/mbagusaditya/scrobbles-cli.git](https://github.com/mbagusaditya/scrobbles-cli.git)
cd scrobbles-cli

# Install dependencies
go mod download

# Build binary with release metadata
./build.sh

```

Or install directly via Go:

```bash
go install [github.com/mbagusaditya/scrobbles-cli@latest](https://github.com/mbagusaditya/scrobbles-cli@latest)

```

---

## Configuration

`scrobbles-cli` resolves configuration files in the following priority order:

1. Current working directory (`./.env` and `./aliases.json`)
2. Standard XDG Base Directory (`~/.config/scrobbles/config.env` and `~/.config/scrobbles/aliases.json`)

Copy the example environment file to your preferred location:

```bash
mkdir -p ~/.config/scrobbles
cp .env.example ~/.config/scrobbles/config.env

```

Configuration variables:

```env
DATABASE_URL=libsql://your-database-name.turso.io
DATABASE_AUTH_TOKEN=your-turso-auth-token
LASTFM_API_KEY=your-lastfm-api-key
LASTFM_SHARED_SECRET=your-lastfm-shared-secret
LASTFM_USERNAME=your-lastfm-username

```

---

## Usage

### 1. Synchronize Scrobbles (`sync`)

```bash
# Incremental sync from latest recorded scrobble to current time
scrobbles sync

# Sync preset time ranges
scrobbles sync --range day
scrobbles sync --range week

# Sync a specific custom range (max 7 days per batch)
scrobbles sync --from 2026-09-01 --to 2026-09-07

```

### 2. Normalize Master Catalog & Track Durations (`resolve`)

Map raw logs into master `artists` and `tracks` tables and fetch actual track durations from Last.fm:

```bash
# Link unlinked scrobble logs (default batch size: 500)
scrobbles resolve

# Link unlinked logs AND fetch missing track durations from Last.fm API
scrobbles resolve --durations

# Run with custom batch size
scrobbles resolve -b 1000 -d

```

### 3. Manage Track Title Aliases (`alias`)

Consolidate statistics for track title variants locally without altering raw database logs:

```bash
# Add a track title alias rule
scrobbles alias add --artist "MyGO!!!!!" --from "Sunday" --to "騒混出"

# List all configured alias rules
scrobbles alias list

# Remove an alias rule by its list index
scrobbles alias remove 1

```

### 4. Display Statistics (`stats`)

```bash
# Show all-time statistics with actual track durations
scrobbles stats

# Show statistics for preset periods
scrobbles stats --day
scrobbles stats --week
scrobbles stats --month
scrobbles stats --year

# Show statistics for a custom range
scrobbles stats --from 2026-08-01 --to 2026-08-31

```

### 5. Top Rankings, Library & Metadata Info (`top`, `list`, `info`)

```bash
# View top tracks, artists, or albums
scrobbles top track
scrobbles top artist --month -n 15
scrobbles top album --year

# Inspect recent scrobble history or normalized master libraries
scrobbles list
scrobbles list track
scrobbles list artist
scrobbles list album

# View detailed Last.fm metadata + personal play history
scrobbles info track "MyGO!!!!!" "孤壊牢"
scrobbles info artist "MyGO!!!!!"

```

### 6. Check Version Metadata

```bash
scrobbles version

```
