package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func colorizeDir(dir string, sepStyle lipgloss.Style, dirStyle lipgloss.Style, width int) string {
	var dirBuilder strings.Builder
	start := 0
	for i := 0; i < len(dir); i++ {
		if dir[i] == '/' || dir[i] == '\\' {
			if start < i {
				dirBuilder.WriteString(dirStyle.Render(dir[start:i]))
			}
			dirBuilder.WriteString(sepStyle.Render(dir[i : i+1]))
			start = i + 1
		}
	}
	if start < len(dir) {
		dirBuilder.WriteString(dirStyle.Render(dir[start:]))
	}
	return ansi.Truncate(dirBuilder.String(), width, "…")
}
