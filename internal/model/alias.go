package model

// TrackAlias merepresentasikan satu aturan pemetaan judul lagu varian ke judul utama (kanonik).
type TrackAlias struct {
	Artist string `json:"artist"`
	From   string `json:"from"` // Judul varian/lama (misal: "孤壊牢 - KOKORO")
	To     string `json:"to"`   // Judul utama/target (misal: "孤壊牢")
}

// AliasConfig adalah struktur root file aliases.json.
type AliasConfig struct {
	Tracks []TrackAlias `json:"tracks"`
}
