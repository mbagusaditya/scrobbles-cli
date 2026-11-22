package model

import "time"

// TopItem merepresentasikan entitas baris peringkat statistik.
type TopItem struct {
	Name   string
	Detail string // Opsional: nama artis untuk trek/album
	Count  int
}

type TopFilter struct {
	From  time.Time
	To    time.Time
	Limit int
}
