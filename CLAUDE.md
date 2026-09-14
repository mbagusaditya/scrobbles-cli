# Project: Scrobbles CLI (Go + Turso)

## Konteks

CLI untuk manajemen data personal (anime, scrobble musik Last.fm, catatan, dll) yang berbagi database **Turso/SQLite** dengan web app (Drizzle ORM).
CLI Go ini bertindak sebagai **data ingestion, background synchronization, dan maintenance engine** untuk domain musik/scrobble Last.fm.

## Tech Stack

- **Bahasa**: Go 1.25
- **Database**: Turso (libSQL), driver native `tursodatabase/libsql-client-go` (pure Go, no CGO)
- **CLI Framework**: `spf13/cobra`
- **Config**: `.env` via `joho/godotenv`
- **UUID**: `google/uuid` (UUID v4 string 36 karakter, konsisten dengan `crypto.randomUUID()` di Drizzle)

## Module: `github.com/mbagusaditya/scrobbles-cli`

## Schema Relevan (Drizzle ORM Web — Read-Only Reference)

- `artists`: `id (UUID PK)`, `name (TEXT UNIQUE)`, `mbid`, `imageUrl`, `createdAt`
- `tracks`: `id (UUID PK)`, `artistId (FK → artists)`, `title`, `albumName`, `durationMs`, `coverUrl`, `createdAt`
    - Unique Constraint: `(artistId, title, albumName)`
- `scrobble_logs`: `id (UUID PK)`, `trackId (FK nullable → tracks)`, `rawTitle`, `rawArtist`, `rawAlbum`, `playedAt (Unix Epoch Seconds)`, `createdAt`
    - Unique Index: `(rawArtist, rawTitle, playedAt)`
    - **Partial Indexes**:
        - `idx_scrobble_logs_unresolved`: `(rawArtist, rawTitle) WHERE trackId IS NULL` (optimasi resolver)
        - `idx_scrobble_logs_valid_albums`: `(rawAlbum) WHERE rawAlbum IS NOT NULL AND rawAlbum != ''` (optimasi distinct album stats)

**Catatan Timestamp**: SQLite Turso menyimpan waktu sebagai Unix Epoch integer (detik). Di Go, konversi manual menggunakan `.Unix()` saat write dan `time.Unix()` saat read.

## Arsitektur (4 Layer)

```text
scrobbles-cli/
├── .env.example
├── go.mod
├── main.go
├── cmd/
│   ├── root.go              → wiring dependency (config → db → lastfm → service)
│   ├── sync.go              → subcommand sync (--range latest|day|week, --from, --to)
│   ├── list.go              → subcommand list log scrobble dengan filter tanggal & limit
│   ├── stats.go             → subcommand agregasi stats (preset --day/week/month/year, custom range, all-time)
│   ├── resolve.go           → subcommand deduplikasi & normalisasi master artists/tracks
│   └── version.go           → subcommand informasi binary, commit, & build date via ldflags
└── internal/
    ├── config/config.go     → validasi environment variables
    ├── model/               → entity murni tanpa DB logic (Artist, Track, Scrobble)
    ├── lastfm/              → HTTP client, rate throttle (500ms), retry exponential backoff
    ├── db/db.go             → koneksi database Turso murni
    └── service/
        ├── scrobble_service.go  → ingestion, fetch pagination, insert ignore
        ├── stat_service.go      → kalkulasi ringkasan metrik (all-time & ranged)
        └── resolver_service.go  → batch processing unlinked logs -> master artists/tracks
```

**Prinsip pembagian layer** (disepakati eksplisit):

- `model` = entity murni, tidak tahu DB/API
- `lastfm` = HTTP client + build URL, tidak ada business logic
- `db` = cuma koneksi, tidak ada query
- `service` = semua business logic (mapping, orkestrasi, query SQL)
- `cmd` = parsing flag, orkestrasi command, printing ke terminal

## Keputusan Desain Kunci

| Area                           | Keputusan                                                                                                                                                                 |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Resolver track/artist matching | **Aktif (Batch 500)** — Memanfaatkan partial index `WHERE track_id IS NULL`. `INSERT OR IGNORE` ke `artists` & `tracks`, lalu update balik `track_id` di `scrobble_logs`. |     |
| Dedup insert                   | `INSERT OR IGNORE` memanfaatkan unique index yang sudah ada di schema                                                                                                     |
| Retry API                      | Exponential backoff (500ms, 1s, 2s, 4s...), retry hanya untuk network error/429/5xx, max 4x retry                                                                         |
| Timeout                        | Per-request saja (15 detik di `http.Client`), tidak ada timeout keseluruhan command                                                                                       |
| Custom range                   | Maksimal 7 hari (`service.MaxCustomRangeDays`), divalidasi & ditolak keras di `cmd` kalau lebih                                                                           |
| Album kosong dari API          | Disimpan sebagai `NULL` (bukan string kosong)                                                                                                                             |
| Now-playing track dari API     | Di-skip saat sync (belum final scrobble, belum ada timestamp)                                                                                                             |
| Relasi antar entity di Go      | Tidak pakai nested struct/pointer ala ORM — cuma foreign key ID; kalau butuh JOIN, buat struct baru khusus untuk hasil query itu                                          |

## Library yang Dipakai (go.mod)

- github.com/spf13/cobra v1.10.2
- github.com/tursodatabase/libsql-client-go (native driver)
- github.com/joho/godotenv v1.5.1
- github.com/google/uuid v1.6.0
