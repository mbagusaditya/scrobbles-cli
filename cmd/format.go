package cmd

import (
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
