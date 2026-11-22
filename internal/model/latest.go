package model

import "time"

// LatestItem merepresentasikan riwayat pemutaran terakhir dari artis, album, atau trek.
type LatestItem struct {
	Name     string
	Detail   string // Nama artis (jika targetnya track/album)
	PlayedAt time.Time
}
