# Scrobbles CLI

A high-performance CLI tool written in Go to ingest, synchronize, normalize, and inspect personal Last.fm scrobble history into a Turso (libSQL) database.

Designed to share a unified database schema with an existing web application (built with Drizzle ORM), serving as a decoupled background ingestion and maintenance engine.

---

## Features

- **Robust Ingestion (`sync`)**: Fetches scrobbles with cursor pagination, 500ms API rate throttling, and exponential backoff retry.
- **Idempotent Ingestion**: Leverages SQLite unique compound indexes (`raw_artist`, `raw_title`, `played_at`) with `INSERT OR IGNORE` to prevent duplicates.
- **Entity Normalization (`resolve`)**: Resolves raw scrobble logs into normalized master `artists` and `tracks` tables using batch transactions and partial index filtering (`WHERE track_id IS NULL`).
- **Instant Aggregation (`stats`)**: Optimized metric aggregation (all-time metrics read directly from master tables, ranged metrics accelerated by partial indexes).
- **History Inspection (`list`)**: Query scrobble history with time-range filtering and custom limits.
- **Zero CGO**: Built with `tursodatabase/libsql-client-go` for easy cross-compilation and static binary distributions.

---

## Architecture Overview

Employs a strict 4-layer architecture:

```text
cmd/        → CLI orchestration (Cobra), flag parsing, ANSI terminal output
internal/
├── service/ → Core business logic, batch orchestration, SQL execution
├── model/   → Pure domain entities (Artist, Track, Scrobble) with UUID generation
├── lastfm/  → HTTP client, URL construction, rate limiting, retry backoff
├── db/      → Turso connection lifecycle management
└── config/  → Environment validation (.env)
```

## Getting Started

### Prerequisites

    Go 1.25 or higher

    A Turso database instance

    Last.fm API credentials

### Installation

Clone the repository and build the binary:

```bash
git clone [https://github.com/mbagusaditya/scrobbles-cli.git](https://github.com/mbagusaditya/scrobbles-cli.git)
cd scrobbles-cli

# Install dependencies
go mod download

# Build binary with release metadata
go build -ldflags="-s -w" -o scrobbles main.go
```

Or install directly via Go:

```bash
go install [github.com/mbagusaditya/scrobbles-cli@latest](https://github.com/mbagusaditya/scrobbles-cli@latest)
```

## Configuration

Copy the example environment file and populate your credentials:

```bash
cp .env.example .env
```

.env configuration:

```cuplikan kode
DATABASE_URL=libsql://your-database-name.turso.io
DATABASE_AUTH_TOKEN=your-turso-auth-token
LASTFM_API_KEY=your-lastfm-api-key
LASTFM_SHARED_SECRET=your-lastfm-shared-secret
LASTFM_USERNAME=your-lastfm-username
```

## Usage

### 1. Synchronize Scrobbles

```bash
# Incremental sync from latest recorded scrobble to current time
scrobbles sync

# Sync preset time ranges
scrobbles sync --range day
scrobbles sync --range week

# Sync a specific custom range (max 7 days per batch)
scrobbles sync --from 2026-09-01 --to 2026-09-07
```

### 2. Normalize Master Catalog

Map raw logs into master artists and tracks tables and link foreign keys:

```bash
# Run resolver with default batch size (500)
scrobbles resolve

# Run with custom batch size
scrobbles resolve -b 1000
```

### 3. Display Statistics

```bash
# Show all-time statistics (zero full-table scan, reads master tables)
scrobbles stats

# Show statistics for preset periods
scrobbles stats --day
scrobbles stats --week
scrobbles stats --month
scrobbles stats --year

# Show statistics for a custom range
scrobbles stats --from 2026-08-01 --to 2026-08-31
```

### 4. Inspect Scrobble History

```bash
# List latest 20 scrobbles
scrobbles list

# List with custom limit and date filtering
scrobbles list -n 50 --from 2026-09-01
```

### 5. Check Version Metadata

```bash
scrobbles version
```
