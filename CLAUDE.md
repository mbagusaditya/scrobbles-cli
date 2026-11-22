# Project: Scrobbles CLI (Go + Turso)

## Konteks

CLI untuk manajemen data personal (scrobble musik Last.fm, dll) yang berbagi database **Turso/SQLite** dengan web app (Drizzle ORM).
CLI Go ini bertindak sebagai **data ingestion, background synchronization, metadata enrichment, dan maintenance engine** untuk domain musik/scrobble Last.fm.

## Tech Stack

- **Bahasa**: Go 1.25
- **Database**: Turso (libSQL), driver native `tursodatabase/libsql-client-go` (pure Go, no CGO)
- **CLI Framework**: `spf13/cobra`
- **Config**: `.env` & XDG Base Directory (`~/.config/scrobbles/config.env`, `~/.config/scrobbles/aliases.json`) via `joho/godotenv`
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
│   ├── root.go              → wiring dependency (config → alias → db → lastfm → service)
│   ├── sync.go              → subcommand sync (--range latest|day|week, --from, --to)
│   ├── list.go              → subcommand list log scrobble & master library (track/artist/album)
│   ├── top.go               → subcommand ranking top tracks, artists, & albums
│   ├── info.go              → subcommand detail metadata & riwayat personal track/artist/album
│   ├── stats.go             → subcommand agregasi stats (durasi aktual, preset --day/week/month/year, custom range)
│   ├── alias.go             → subcommand pengelolaan mapping alias judul lagu lokal (aliases.json)
│   ├── resolve.go           → subcommand normalisasi master artists/tracks & sync durasi (--durations)
│   └── version.go           → subcommand informasi binary, commit, & build date via ldflags
└── internal/
    ├── config/
    │   ├── config.go        → resolusi & validasi environment variables (.env / XDG config.env)
    │   └── alias.go         → persistensi JSON & resolusi path aliases.json (CWD / XDG)
    ├── model/               → entity murni tanpa DB logic (Artist, Track, Scrobble, Alias, Top, Library, Info)
    ├── lastfm/              → HTTP client (getRecentTracks, track/artist/album.getInfo), rate throttle, retry backoff
    ├── db/db.go             → koneksi database Turso murni
    └── service/
        ├── alias_service.go     → lookup in-memory O(1), validasi siklus, & resolusi judul kanonik
        ├── scrobble_service.go  → ingestion, fetch pagination, insert ignore, list logs
        ├── library_service.go   → daftar master unik artists, tracks (alias-aware), & albums
        ├── top_service.go       → kalkulasi peringkat top tracks (alias-aware), artists, & albums
        ├── info_service.go      → gabungan metadata Last.fm API + statistik personal DB
        ├── stat_service.go      → kalkulasi metrik, hari inklusif, & akumulasi durasi aktual (fallback 3.5m)
        └── resolver_service.go  → batch linking unlinked logs -> master tables & sync duration_ms
```

**Prinsip pembagian layer**:

- `model` = entity murni, tidak tahu DB/API
- `lastfm` = HTTP client + build URL, tidak ada business logic
- `db` = cuma koneksi, tidak ada query
- `service` = semua business logic (mapping, orkestrasi, query SQL)
- `cmd` = parsing flag, orkestrasi command, printing ke terminal

## Keputusan Desain Kunci

| Area                           | Keputusan                                                                                                                                                                    |
| ------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Resolusi Konfigurasi (XDG)     | Prioritas 1: file di CWD (`.env`, `aliases.json`). Prioritas 2: XDG Base Directory (`~/.config/scrobbles/config.env`, `~/.config/scrobbles/aliases.json`).                   |
| Track Title Alias Mapping      | **Read-time in-memory** via `AliasService` (`aliases.json`). Mendukung pemetaan _many-to-one_ & _transitive_ tanpa memodifikasi data mentah di database Turso.               |
| Kalkulasi Durasi Listening     | Menggunakan `tracks.duration_ms` aktual (diisi via `resolve --durations` dari `track.getInfo`), dengan fallback `210.000 ms` (3,5 menit) jika `duration_ms` belum tersedia.  |
| Kalkulasi Hari Kalender        | Menggunakan titik tengah hari (`12:00:00` lokal) secara inklusif agar kebal terhadap selisih jam/menit antar batas `played_at`.                                              |
| Resolver & Duration Enrichment | **Aktif (Batch 500)** — Memanfaatkan partial index `WHERE track_id IS NULL` untuk linking, serta `SyncTrackDurations` (rate-limit 300ms) untuk mengisi `tracks.duration_ms`. |
| Dedup insert                   | `INSERT OR IGNORE` memanfaatkan unique index yang sudah ada di schema                                                                                                        |
| Retry API                      | Exponential backoff (500ms, 1s, 2s, 4s...), retry hanya untuk network error/429/5xx, max 4x retry                                                                            |
| Timeout                        | Per-request saja (15 detik di `http.Client`), tidak ada timeout keseluruhan command                                                                                          |
| Custom range                   | Maksimal 7 hari (`service.MaxCustomRangeDays`), divalidasi & ditolak keras di `cmd` kalau lebih                                                                              |
| Album kosong dari API          | Disimpan sebagai `NULL` (bukan string kosong)                                                                                                                                |
| Now-playing track dari API     | Di-skip saat sync (belum final scrobble, belum ada timestamp)                                                                                                                |
| Relasi antar entity di Go      | Tidak pakai nested struct/pointer ala ORM — cuma foreign key ID; kalau butuh JOIN, buat struct baru khusus untuk hasil query itu                                             |

## Library yang Dipakai (go.mod)

- `github.com/spf13/cobra` v1.10.2
- `github.com/tursodatabase/libsql-client-go` (native driver)
- `github.com/joho/godotenv` v1.5.1
- `github.com/google/uuid` v1.6.0
