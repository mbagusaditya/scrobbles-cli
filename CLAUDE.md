# Project: Scrobbles CLI (Go + Turso)

## Konteks

CLI untuk mengurus data pribadi (anime, scrobble musik Last.fm, dll) yang sebelumnya sudah punya web app pakai **Drizzle ORM + SQLite/Turso**. CLI ini dibuat terpisah pakai **Go**, fokus v1 ke domain **scrobble Last.fm**, terhubung ke database Turso yang sama dengan web app.

## Tech Stack

- **Bahasa**: Go 1.25
- **Database**: Turso (libSQL), pakai **native driver** `tursodatabase/libsql-client-go` (no CGO, tanpa embedded replica)
- **CLI framework**: `spf13/cobra`
- **Config**: `.env` file via `joho/godotenv`
- **UUID**: `google/uuid` (ID di-generate di level aplikasi, bukan default DB, konsisten dengan pola `crypto.randomUUID()` di Drizzle)

## Module: `github.com/mbagusaditya/scrobbles-cli`

## Schema Relevan (dari Drizzle, sisi web — read-only reference)

Tabel yang berhubungan dengan domain musik:

- `artists` — id, name (unique), mbid, imageUrl, createdAt
- `tracks` — id, artistId (FK), title, albumName, durationMs, coverUrl, createdAt (album BUKAN tabel terpisah, cuma kolom)
- `scrobble_logs` — id, trackId (FK nullable → tracks), rawTitle, rawArtist, rawAlbum, playedAt, createdAt. Unique index `(rawArtist, rawTitle, playedAt)` untuk cegah duplikat dari Last.fm.

**Catatan penting**: kolom `mode: 'timestamp'` di Drizzle SQLite disimpan sebagai **unix epoch integer (detik)**, bukan string ISO — CLI Go ini menyesuaikan (pakai `.Unix()` manual saat insert, `time.Unix()` saat baca).

## Arsitektur (4 Layer)

scrobbles-cli/
├── .env.example
├── go.mod
├── main.go
├── cmd/
│ ├── root.go → root command + wiring dependency (config→db→lastfm→service)
│ └── sync.go → subcommand sync (flags: --range, --from, --to)
└── internal/
├── config/config.go → load & validasi .env
├── model/
│ ├── helper.go → toNullString() shared helper
│ ├── artist.go → struct Artist + NewArtist()
│ ├── track.go → struct Track + NewTrack()
│ └── scrobble.go → struct Scrobble + NewScrobble()
├── lastfm/
│ ├── types.go → response types Last.fm API (handle gotcha: single-object vs array track)
│ └── client.go → HTTP client, retry exponential backoff, build URL
├── db/db.go → HANYA Connect(), tidak ada query logic
└── service/
└── scrobble_service.go → business logic: mapping API→entity, orkestrasi sync, query DB

**Prinsip pembagian layer** (disepakati eksplisit):

- `model` = entity murni, tidak tahu DB/API
- `lastfm` = HTTP client + build URL, tidak ada business logic
- `db` = cuma koneksi, tidak ada query
- `service` = semua business logic (mapping, orkestrasi, query SQL)
- `cmd` = parsing flag, orkestrasi command, printing ke terminal

## Keputusan Desain Kunci

| Area                           | Keputusan                                                                                                                        |
| ------------------------------ | -------------------------------------------------------------------------------------------------------------------------------- |
| Resolver track/artist matching | **Ditunda** — `trackId` di `scrobble_logs` selalu NULL untuk sekarang                                                            |
| Dedup insert                   | `INSERT OR IGNORE` memanfaatkan unique index yang sudah ada di schema                                                            |
| Retry API                      | Exponential backoff (500ms, 1s, 2s, 4s...), retry hanya untuk network error/429/5xx, max 4x retry                                |
| Timeout                        | Per-request saja (15 detik di `http.Client`), tidak ada timeout keseluruhan command                                              |
| Custom range                   | Maksimal 7 hari (`service.MaxCustomRangeDays`), divalidasi & ditolak keras di `cmd` kalau lebih                                  |
| Album kosong dari API          | Disimpan sebagai `NULL` (bukan string kosong)                                                                                    |
| Now-playing track dari API     | Di-skip saat sync (belum final scrobble, belum ada timestamp)                                                                    |
| Relasi antar entity di Go      | Tidak pakai nested struct/pointer ala ORM — cuma foreign key ID; kalau butuh JOIN, buat struct baru khusus untuk hasil query itu |

## Status Implementasi

✅ **Selesai:**

- `internal/model` — `Artist`, `Track`, `Scrobble` (entity + constructor dengan auto UUID/timestamp)
- `internal/config` — load & validasi `.env` (`DATABASE_URL`, `DATABASE_AUTH_TOKEN`, `LASTFM_API_KEY`, `LASTFM_SHARED_SECRET`, `LASTFM_USERNAME`)
- `internal/lastfm` — client dengan retry + support parameter `from`/`to`
- `internal/db` — `Connect()`
- `internal/service` — `ScrobbleService` dengan method:
    - `SyncLatest(ctx, onProgress)` — incremental dari scrobble terakhir di DB sampai sekarang
    - `FetchRange(ctx, from, to, onProgress)` — inti mekanisme (pagination + insert), dipakai semua mode
    - `ProgressFunc`/`PageProgress` — callback progress per halaman (service tidak print sendiri)
- `cmd/root.go` — wiring dependency + `Execute()`
- `cmd/sync.go` — subcommand dengan flag `--range latest|day|week` dan `--from`/`--to` custom range

⏳ **Belum dikerjakan:**

- `cmd/list.go` — lihat data scrobble dengan filter
- Command stats/agregasi (top artist/track/album)
- Resolver matching `scrobble_logs.trackId` → `tracks`/`artists`
- Struct `TrackWithArtist` dsb untuk hasil query JOIN (kalau nanti dibutuhkan `list`)

## Isu Terbuka (Belum Diputuskan)

1. **Custom range `--to` inclusive/exclusive**: saat ini `--to 2026-09-05` di-parse sebagai jam 00:00 tanggal itu (pakai `time.Local`), sehingga scrobble di tanggal 5 September sendiri **tidak ikut ke-fetch**. Perlu diputuskan apakah perlu di-adjust ke akhir hari (`23:59:59`) atau dibiarkan seperti sekarang.
2. **Kegagalan 1 halaman saat sync**: saat ini kalau ada 1 halaman gagal terus setelah retry habis, seluruh proses `sync` berhenti total (error), bukan skip halaman itu dan lanjut ke halaman berikutnya.
3. **Timezone custom range**: parsing tanggal pakai `time.Local` (timezone mesin yang menjalankan CLI) — perlu dipastikan ini sesuai ekspektasi.

## Library yang Dipakai (go.mod)

github.com/spf13/cobra v1.10.2
github.com/tursodatabase/libsql-client-go (native driver)
github.com/joho/godotenv v1.5.1
github.com/google/uuid v1.6.0
