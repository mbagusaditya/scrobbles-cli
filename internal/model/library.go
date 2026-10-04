package model

// LibraryItem merepresentasikan satu entitas unik di pustaka (artist, album, atau track).
type LibraryItem struct {
	Name       string // Nama track / artist / album
	Detail     string // Nama artist (untuk track dan album)
	TotalPlays int    // Total scrobble sepanjang waktu
}

// LibraryFilter menampung parameter pagination untuk listing katalog.
type LibraryFilter struct {
	Page  int
	Limit int
}

// LibraryResult membungkus daftar item beserta metadata paginasi untuk kebutuhan UI.
type LibraryResult struct {
	Items      []LibraryItem
	TotalItems int
	Page       int
	Limit      int
	TotalPages int
}
