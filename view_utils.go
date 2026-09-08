package main

import (
	"strings"

	"mc/internal/paint"
)

func colorizeDir(dir string, sepStyle paint.Style, dirStyle paint.Style, width int) string {
	return colorizeDirHover(dir, sepStyle, dirStyle, dirStyle, width, -1)
}

func colorizeDirHover(dir string, sepStyle, dirStyle, hoverStyle paint.Style, width, hoverX int) string {
	var dirBuilder strings.Builder
	component := strings.Builder{}
	x := 0
	flush := func() {
		text := component.String()
		if text == "" {
			return
		}
		style := dirStyle
		componentWidth := paint.Width(text)
		if hoverX >= x && hoverX < x+componentWidth {
			style = hoverStyle
		}
		dirBuilder.WriteString(style.Render(text))
		x += componentWidth
		component.Reset()
	}
	for _, r := range dir {
		if r == '/' || r == '\\' {
			flush()
			dirBuilder.WriteString(sepStyle.Render(string(r)))
			x++
		} else {
			component.WriteRune(r)
		}
	}
	flush()
	return truncate(dirBuilder.String(), width)
}

func truncate(s string, width int) string {
	return paint.Truncate(s, width)
}

// Match terminal cells, including wide characters and combining sequences.
func breadcrumbAtX(dir string, x, width int) string {
	if x < 0 || x >= width || (paint.Width(dir) > width && x >= width-1) {
		return ""
	}
	start := 0
	for end := 0; end <= len(dir); end++ {
		if end < len(dir) && dir[end] != '\\' && dir[end] != '/' {
			continue
		}
		if start < end && x >= paint.Width(dir[:start]) && x < paint.Width(dir[:end]) {
			target := dir[:end]
			if len(target) == 2 && target[1] == ':' {
				target += `\`
			}
			return target
		}
		start = end + 1
	}
	return ""
}
