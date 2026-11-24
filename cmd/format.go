package cmd

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

func padRight(s string, width int) string {
	visualWidth := runewidth.StringWidth(s)
	if visualWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visualWidth)
}

// calcColumnWidth menghitung lebar kolom visual optimal berdasarkan daftar string,
// dibatasi di antara minWidth dan maxWidth.
func calcColumnWidth(items []string, minWidth, maxWidth int) int {
	width := minWidth
	for _, s := range items {
		if w := runewidth.StringWidth(s); w > width {
			width = w
		}
	}
	if width > maxWidth {
		return maxWidth
	}
	return width
}

// formatNumber memformat angka integer dengan pemisah ribuan titik (1234567 -> 1.234.567).
func formatNumber(n int) string {
	if n < 0 {
		return fmt.Sprintf("-%s", formatNumber(-n))
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var res []byte
	rem := len(s) % 3
	if rem > 0 {
		res = append(res, s[:rem]...)
	}

	for i := rem; i < len(s); i += 3 {
		if len(res) > 0 {
			res = append(res, '.')
		}
		res = append(res, s[i:i+3]...)
	}

	return string(res)
}
