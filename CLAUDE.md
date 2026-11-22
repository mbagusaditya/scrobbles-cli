# Project Architecture & Engineering Guidelines

## Project Overview

`scrobbles-cli` is a Go CLI application for syncing, normalizing,
enriching, and analyzing personal Last.fm listening history backed by
Turso (libSQL) or local SQLite.

- **Module:** `github.com/mbagusaditya/scrobbles-cli`
- **Language:** Go (Cobra CLI framework)
- **Database Engine:** Turso (libSQL) / SQLite via standard
  `database/sql` (`*sql.DB`)

## Core Architecture & Request Flow

Every command follows a strict unidirectional dependency chain:

```text
user ──► cmd/root.go ──► specific cmd/*.go ──► App (Lazy DI) ──► service ──► repository ──► *sql.DB / JSON
```

### Directory Structure

```text
scrobbles-cli/
├── main.go                       # Entry point calling cmd.Execute()
├── cmd/
│   ├── app.go                    # Lazy DI Container (App struct) caching DB, clients, & services
│   ├── root.go                   # Root Cobra command & PersistentPostRunE cleanup (app.Close())
│   ├── alias.go                  # Track title normalization rules (JSON-backed, zero DB)
│   ├── info.go                   # Entity metadata (Last.fm) + personal play stats
│   ├── latest.go                 # Most recently played artists, albums, and tracks
│   ├── list.go                   # Raw scrobble logs & master catalog (artists/albums/tracks)
│   ├── resolve.go                # Entity linking (scrobble_logs -> artists/tracks) & duration sync
│   ├── stats.go                  # Listening summary, duration totals, peak day/hour, repeat ratio
│   ├── sync.go                   # Incremental & custom date-range scrobble ingestion from Last.fm
│   ├── top.go                    # Top tracks, artists, and albums leaderboard
│   └── version.go                # CLI version info
└── internal/
    ├── config/                   # Environment/config file loading & alias path resolution
    ├── db/                       # Database connection factory (*sql.DB for Turso / SQLite)
    ├── lastfm/                   # Last.fm HTTP API client & response DTOs
    ├── model/                    # Pure domain models, filters, and result DTOs (zero DB/HTTP deps)
    │   ├── alias.go
    │   ├── info.go
    │   ├── latest.go
    │   ├── library.go
    │   ├── resolver.go
    │   ├── scrobble.go
    │   ├── scrobble_sync.go
    │   ├── stats.go
    │   └── top.go
    ├── repository/               # Unified persistence layer (interfaces + *sql.DB / JSON impls)
    │   ├── alias_repository.go   # JSON file persistence for track aliases
    │   ├── info_repository.go    # Conditional CASE WHEN entity & library stats
    │   ├── latest_repository.go  # Latest played aggregations
    │   ├── library_repository.go # Paginated unique catalog queries
    │   ├── resolver_repository.go# Batch transactional entity linking
    │   ├── scrobble_repository.go# Raw scrobble insertion & filtered listing
    │   ├── stats_repository.go   # O(1) memory streaming row projection
    │   ├── top_repository.go     # Ranked aggregation queries
    │   └── track_repository.go   # Master tracks metadata & duration_ms persistence
    └── service/                  # Business logic, alias merging, & external API orchestration
        ├── alias_service.go
        ├── info_service.go
        ├── latest_service.go
        ├── library_service.go
        ├── resolver_service.go
        ├── scrobble_service.go
        ├── stats_service.go
        └── top_service.go
```

## Architectural Conventions & Design Rules

### 1. Unified `internal/repository/` Package

- **Do NOT** split repositories into driver sub-packages
  (`repository/sqlite`, `repository/turso`).
- Both local SQLite and Turso share the exact same SQLite SQL dialect
  and `database/sql` interface.
- Connection routing (`file:...` vs `libsql://...`) is handled
  exclusively inside `internal/db`. Every SQL repository accepts
  `*sql.DB` via its `New<Domain>Repository(db *sql.DB)` constructor.

### 2. Lazy Dependency Injection (`cmd/app.go`)

- `cmd/root.go` must **never** open a database connection in
  `PersistentPreRunE`.
- Lightweight commands (`version`, `help`, `alias`) must execute
  without triggering `db.Connect`.
- Each subcommand validates CLI flags first, then retrieves its
  required service via `app.<Domain>Service()`.
- `App.Close()` is called once in `rootCmd.PersistentPostRunE` and
  safely closes `*sql.DB` only if it was initialized.

### 3. Track Title Canonicalization (`AliasService`)

- Track title aliases are stored locally in `aliases.json`
  (`AliasRepository`) and indexed in-memory with multi-hop cycle
  protection (`maxAliasResolutionHops = 5`).
- Any domain aggregating or querying tracks (`ScrobbleService`,
  `TopService`, `LibraryService`, `LatestService`, `InfoService`,
  `StatsService`, `ResolverService`) must normalize track titles
  through `AliasService.ResolveTrackTitle(artist, title)` (or
  `ExpandTrackTitles` for multi-variant lookup).
- For track rankings/lists (`TopTracks`, `ListLibraryTracks`,
  `GetLatestTracks`), raw `(artist, title)` counts/timestamps are
  fetched unpaginated from SQL, merged by canonical key in Go, sorted,
  and sliced in memory so alias variants never split across separate
  entries.

### 4. Query & Memory Discipline

- **No N+1 Queries in Batches:**
  `ResolverRepository.LinkUnresolvedBatch` preloads existing `artists`
  and `tracks` into Go maps within the transaction (`2 SELECT`s total)
  and uses comparable Go structs (`trackLookupKey`) as map keys
  instead of string-concatenated composite keys.
- **Single-Roundtrip Conditional Stats:** `InfoRepository` computes
  total library scrobbles alongside entity play counts,
  `MIN(played_at)`, and `MAX(played_at)` in a single `CASE WHEN` query
  to minimize Turso network latency.
- **Streaming Stats Aggregation:**
  `StatsRepository.StreamScrobbleRows` streams rows via callback
  rather than loading all `scrobble_logs` into a slice, keeping memory
  usage constant.

### 5. Naming & Layer Boundaries

- **Models (`internal/model`):** Input filters (`*Filter`) and output
  DTOs (`*Result`, `*Item`) live in `internal/model`, never in
  `internal/service`.
- **Services (`internal/service`):** Zero raw SQL strings or
  `database/sql` imports allowed inside `internal/service`.
- **Plurality:** Use `stats` (plural) consistently across
  `StatsRepository`, `StatsService`, `stats_repository.go`,
  `stats_service.go`, and `cmd/stats.go`.
