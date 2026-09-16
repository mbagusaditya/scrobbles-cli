package model

// TopItem merepresentasikan entitas baris peringkat statistik.
type TopItem struct {
	Name   string
	Detail string // Opsional: nama artis untuk trek/album
	Count  int
}
