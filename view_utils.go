package main

import (
	"strings"

	"mc/internal/paint"
)

func colorizeDir(dir string, sepStyle paint.Style, dirStyle paint.Style, width int) string {
	return colorizeDirHover(dir, sepStyle, dirStyle, dirStyle, width, -1)
}

func colorizeDirHover(dir string, sepStyle, dirStyle, hoverStyle paint.Style, width, hoverX int) string {
	return colorizeDirRoot(dir, sepStyle, dirStyle, hoverStyle, dirStyle, -1, width, hoverX)
}

// colorizeDirRoot renders a breadcrumb with one component -- the one ending at
// byte offset rootEnd, which is where a repository begins -- in a style of its
// own, so the path itself says which part of it is the work tree. A negative
// rootEnd marks no component at all.
func colorizeDirRoot(dir string, sepStyle, dirStyle, hoverStyle, rootStyle paint.Style, rootEnd, width, hoverX int) string {
	var dirBuilder strings.Builder
	component := strings.Builder{}
	x := 0
	flush := func(end int) {
		text := component.String()
		if text == "" {
			return
		}
		style := dirStyle
		if end == rootEnd {
			style = rootStyle
		}
		componentWidth := paint.Width(text)
		// Hovering outranks the root mark: the pointer has to keep showing
		// what a click would do.
		if hoverX >= x && hoverX < x+componentWidth {
			style = hoverStyle
		}
		dirBuilder.WriteString(style.Render(text))
		x += componentWidth
		component.Reset()
	}
	for i, r := range dir {
		if r == '/' || r == '\\' {
			flush(i)
			dirBuilder.WriteString(sepStyle.Render(string(r)))
			x++
		} else {
			component.WriteRune(r)
		}
	}
	flush(len(dir))
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
