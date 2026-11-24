package model

import "time"

// ReplicaDirection menentukan arah transfer data antar SQLite lokal dan Turso.
type ReplicaDirection string

const (
	DirectionPush ReplicaDirection = "push" // Lokal SQLite -> Turso Cloud
	DirectionPull ReplicaDirection = "pull" // Turso Cloud -> Lokal SQLite
)

// ReplicaOptions mengatur perilaku transfer sinkronisasi database.
type ReplicaOptions struct {
	Full      bool // Jika true, memaksa transfer seluruh baris tanpa memotong berdasarkan watermark terakhir
	BatchSize int  // Ukuran batch transaksi untuk setiap tahap (default 500)
}

// ReplicaProgress melaporkan progres transfer per tahap/batch ke layer CLI.
type ReplicaProgress struct {
	Stage     string // "artists", "tracks", atau "scrobble_logs"
	Processed int
	Total     int
}

// ReplicaResult merangkum hasil akhir eksekusi push/pull.
type ReplicaResult struct {
	Direction        ReplicaDirection
	ArtistsProcessed int
	ArtistsInserted  int
	TracksProcessed  int
	TracksInserted   int
	LogsProcessed    int
	LogsInserted     int
	Duration         time.Duration
}

// DBStatsSummary menyimpan ringkasan jumlah baris pada satu instance database.
type DBStatsSummary struct {
	ArtistsCount int
	TracksCount  int
	LogsCount    int
	LatestPlayed time.Time
}

// ReplicaArtist merepresentasikan baris tabel master artists untuk kebutuhan replikasi.
type ReplicaArtist struct {
	ID        string
	Name      string
	MBID      string
	ImageURL  string
	CreatedAt int64
}

// ReplicaTrack merepresentasikan baris tabel master tracks untuk kebutuhan replikasi.
type ReplicaTrack struct {
	ID         string
	ArtistID   string
	Title      string
	AlbumName  string
	DurationMs int64
	HasDur     bool
	CreatedAt  int64
	CoverURL   string
}

// ReplicaScrobbleLog merepresentasikan baris tabel scrobble_logs untuk kebutuhan replikasi.
type ReplicaScrobbleLog struct {
	ID        string
	TrackID   string
	RawTitle  string
	RawArtist string
	RawAlbum  string
	PlayedAt  int64
	CreatedAt int64
}

// TrackNaturalKey digunakan untuk mencocokkan track secara unik lintas database
// meskipun UUID primary key-nya berbeda.
type TrackNaturalKey struct {
	ArtistID string
	Title    string
	Album    string
}
