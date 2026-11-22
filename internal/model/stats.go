package model

import "time"

// StatsFilter menampung parameter filter rentang waktu untuk agregasi statistik.
type StatsFilter struct {
	From time.Time
	To   time.Time
}

// StatsResult menampung hasil lengkap ringkasan statistik mendengarkan.
type StatsResult struct {
	// Ringkasan Utama
	TotalScrobbles int
	TotalArtists   int
	TotalAlbums    int
	TotalTracks    int

	// Durasi Mendengarkan
	TotalDurationMs     int64   // Akumulasi total durasi (aktual + fallback) dalam milidetik
	ActualDurationCount int     // Jumlah scrobble yang memiliki durasi aktual (> 0 ms)
	ActualDurationRatio float64 // Persentase scrobble dengan durasi aktual (0.0 - 100.0)

	// Rentang & Metrik Tambahan
	FirstScrobble time.Time
	LastScrobble  time.Time
	ActiveDays    int
	TotalDays     int
	AvgPerDay     float64
	PeakDayDate   string
	PeakDayHits   int
	PeakHour      int
	RepeatRatio   float64
}
