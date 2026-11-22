package model

// UnresolvedTrackItem merepresentasikan kombinasi unik (artist, title, album)
// dari scrobble_logs yang track_id-nya masih NULL.
type UnresolvedTrackItem struct {
	RawArtist string
	RawTitle  string
	RawAlbum  string
}

// BatchLinkResult melaporkan jumlah entitas baru dan log yang tertaut dalam satu batch.
type BatchLinkResult struct {
	NewArtists  int
	NewTracks   int
	UpdatedLogs int
}

// ResolveProgress melaporkan progres tiap batch pada proses resolve unlinked scrobbles.
type ResolveProgress struct {
	BatchProcessed int
	Remaining      int
	NewArtists     int
	NewTracks      int
	UpdatedLogs    int
}

// ResolveResult merangkum keseluruhan hasil proses resolve unlinked scrobbles.
type ResolveResult struct {
	TotalUpdatedLogs int
	TotalNewArtists  int
	TotalNewTracks   int
}

// MissingDurationTrack merepresentasikan pasangan (artist, title) yang belum memiliki duration_ms.
type MissingDurationTrack struct {
	Artist string
	Title  string
}

// DurationProgress melaporkan progres pengambilan durasi per lagu dari Last.fm.
type DurationProgress struct {
	Current    int
	Total      int
	Artist     string
	Title      string
	DurationMs int64
	Found      bool
}

// DurationSyncResult merangkum keseluruhan hasil sinkronisasi durasi lagu.
type DurationSyncResult struct {
	Processed int
	Updated   int
	NotFound  int
}
