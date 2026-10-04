package model

import "time"

// EntityInfoResult menyimpan gabungan metadata global dan statistik lokal.
type EntityInfoResult struct {
	// Metadata Entitas
	Type   string // "Artist", "Album", atau "Track"
	Name   string
	Artist string // Terisi untuk Album dan Track
	Album  string // Terisi untuk Track jika ada

	// Metadata Global (Last.fm)
	GlobalListeners int
	GlobalPlaycount int
	Tags            []string
	DurationSec     int    // Khusus Track (detik)
	BioOrSummary    string // Ringkasan bio/deskripsi

	// Statistik Personal (Turso)
	PeriodLabel   string
	UserPlaycount int
	TotalLibrary  int     // Total scrobbles di database pada periode ini
	ShareRatio    float64 // Persentase terhadap total scrobbles
	FirstHeard    time.Time
	LastHeard     time.Time
}
