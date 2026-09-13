package main

import (
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"strings"

	"github.com/Fiend3d/catatui"
	"github.com/Fiend3d/catatui/widgets"
	"mc/internal/paint"
)

type vibeVisualRow struct {
	row          int
	prefix, text string
	continuation bool
}

func (v *vibeState) toggleWrap(height int) {
	row, continuation := 0, 0
	if v.start < len(v.visual) {
		row = v.visual[v.start].row
		continuation = v.start - v.rowPosition(row)
	}
	v.noWrap = !v.noWrap
	if v.layoutWidth > 0 {
		v.layout(v.layoutWidth)
		v.start = v.rowPosition(row)
		if !v.noWrap && row+1 < len(v.rowOffsets) {
			v.start += min(continuation, v.rowOffsets[row+1]-v.start-1)
		}
	}
	v.hover = ""
	v.clampView(height)
}
func (v *vibeState) scrollbarTo(y, height int) {
	extent := len(v.visual)
	limit := max(0, extent-height)
	thumb := height
	if extent > 0 {
		thumb = min(height, max(1, (height*height+extent/2)/extent))
	}
	span := height - thumb
	if span <= 0 {
		v.start = 0
		return
	}
	position := min(max(0, y-thumb/2), span)
	v.start = (position*limit + span/2) / span
	v.hover = ""
}

func (v *vibeState) rowPosition(index int) int {
	if index >= 0 && index < len(v.rowOffsets) {
		return v.rowOffsets[index]
	}
	return index
}
func (v *vibeState) rowAtY(y, height int) int {
	if y < 2 || y >= height+2 {
		return -1
	}
	position := v.start + y - 2
	if v.visual != nil {
		if position < len(v.visual) {
			return v.visual[position].row
		}
		return -1
	}
	if position < len(v.rows) {
		return position
	}
	return -1
}
func (v *vibeState) layout(width int) {
	v.layoutWidth = width
	v.visual = nil
	v.rowOffsets = make([]int, len(v.rows))
	connectors := vibeTreePrefixes(v.rows, max(0, (width-24)/2))
	for index, r := range v.rows {
		prefix := "  " + connectors[index] + "  "
		if r.depth == 0 && len(v.rows) > index+1 {
			// Root anchors the tree even though it cannot be collapsed.
			prefix = "  ┬ "
		}
		if r.branch {
			arrow := "▾ "
			if v.collapsed[r.id] {
				arrow = "▸ "
			}
			prefix = "  " + connectors[index] + arrow
		}
		text := compareLiteral(r.text)
		switch r.kind {
		case 'f':
			f := v.snapshot.files[r.file]
			text = fmt.Sprintf("[%s] %s  +%d −%d", f.status, text, f.added, f.deleted)
			if f.oldPath != "" && f.oldPath != f.path {
				text += " ← " + compareLiteral(f.oldPath)
			}
		case 'h':
			text = vibeHunkLabel(v.snapshot.files[r.file].hunks[r.hunk])
		case 'l':
			l := v.snapshot.files[r.file].hunks[r.hunk].lines[r.line]
			old, newLine := "", ""
			if l.old > 0 {
				old = fmt.Sprint(l.old)
			}
			if l.new > 0 {
				newLine = fmt.Sprint(l.new)
			}
			prefix += fmt.Sprintf("%5s %5s %c ", old, newLine, l.kind)
			// Expand tabs before wrapping, preserving source whitespace.
			var literal strings.Builder
			column := 0
			for i, part := range strings.Split(l.text, "\t") {
				if i > 0 {
					padding := 4 - column%4
					literal.WriteString(strings.Repeat(" ", padding))
					column += padding
				}
				literal.WriteString(compareLiteral(part))
				column += ansi.StringWidth(compareLiteral(part))
			}
			text = literal.String()
		}
		v.rowOffsets[index] = len(v.visual)
		available := max(2, width-ansi.StringWidth(prefix))
		wrapped := []string{text}
		if !v.noWrap {
			wrapped = strings.Split(ansi.Wrap(text, available, ""), "\n")
		}
		for i, line := range wrapped {
			p := prefix
			if i > 0 { // Keep ancestor lanes; omit arrows, elbows and line numbers.
				lanes := strings.ReplaceAll(strings.ReplaceAll(connectors[index], "├─", "│ "), "└─", "  ")
				if r.depth == 0 && len(v.rows) > index+1 {
					lanes = "│ "
				}
				p = "  " + lanes + strings.Repeat(" ", max(0, ansi.StringWidth(prefix)-2-ansi.StringWidth(lanes)))
			}
			v.visual = append(v.visual, vibeVisualRow{index, p, line, i > 0})
		}
	}
}

// vibeHunkLabel describes a hunk by where it lands in the current file rather
// than by its raw "@@ -a,b +c,d @@" header. A pure deletion has no current
// lines, so it is placed after the line that precedes it.
func vibeHunkLabel(h vibeHunk) string {
	current, added, deleted := 0, 0, 0
	for _, l := range h.lines {
		switch l.kind {
		case ' ':
			current++
		case '+':
			current++
			added++
		case '-':
			deleted++
		}
	}
	var text string
	switch {
	case current == 0 && h.new == 0:
		text = "Removed at the start"
	case current == 0:
		text = fmt.Sprintf("Removed after line %d", h.new)
	case current == 1:
		text = fmt.Sprintf("Line %d", h.new)
	default:
		text = fmt.Sprintf("Lines %d–%d", h.new, h.new+current-1)
	}
	text += fmt.Sprintf("  +%d −%d", added, deleted)
	// Git appends the enclosing function or section after the closing @@.
	if _, rest, ok := strings.Cut(strings.TrimPrefix(h.header, "@@"), "@@"); ok {
		if context := strings.TrimSpace(rest); context != "" {
			text += "  in " + compareLiteral(context)
		}
	}
	return text
}

// Build connectors from the entire visible tree, so scrolling into a branch
// retains its ancestor lanes and collapsed siblings still get correct elbows.
func vibeTreePrefixes(rows []vibeRow, maxDepth int) []string {
	last := make(map[string]string)
	for _, row := range rows {
		last[row.parent] = row.id
	}
	prefixes := make([]string, len(rows))
	var ancestors []vibeRow
	for i, row := range rows {
		depth := min(row.depth, len(ancestors))
		ancestors = ancestors[:depth]
		var prefix strings.Builder
		for level := 1; level < row.depth && level < maxDepth; level++ {
			ancestor := ancestors[level]
			if last[ancestor.parent] != ancestor.id {
				prefix.WriteString("│ ")
			} else {
				prefix.WriteString("  ")
			}
		}
		if row.depth > 0 && maxDepth > 0 && row.branch {
			if last[row.parent] == row.id {
				prefix.WriteString("└─")
			} else {
				prefix.WriteString("├─")
			}
		} else if row.depth > 0 && maxDepth > 0 {
			prefix.WriteString("  ")
		}
		prefixes[i] = prefix.String()
		ancestors = append(ancestors, row)
	}
	return prefixes
}

func (m *model) drawVibe(f *catatui.Frame, area catatui.Rect) {
	v, t := &m.vibe, m.theme
	w, h := int(area.Width), int(area.Height)
	base := t.baseStyle
	bodyWidth := w - 2
	if v.layoutWidth != bodyWidth {
		v.layout(bodyWidth)
	}
	f.Buffer().SetStyle(area, base.Native())
	v.clampView(max(1, h-3))
	put := func(y int, text string) {
		textAt(f, catatui.NewRect(area.X, area.Y+uint16(y), uint16(bodyWidth), 1), truncate(text, bodyWidth))
	}
	// Chrome rows span the scrollbar column too. Clip the plain text before
	// styling so padding never triggers an ellipsis.
	chrome := func(y int, text string, style paint.Style) {
		textAt(f, catatui.NewRect(area.X, area.Y+uint16(y), area.Width, 1), style.Width(w).Render(truncate(text, w)))
	}
	title := fmt.Sprintf(" Vibe · %s · %d changed files", compareLiteral(v.snapshot.branch), len(v.snapshot.files))
	initial := v.snapshot.fingerprint == [32]byte{}
	if v.loading && initial {
		title += " · refreshing"
	}
	chrome(0, title, base.Bold(true).Foreground(t.accentColor3))
	status := " Live · refreshes every 2s · changes since HEAD + untracked"
	if v.err != "" {
		status = " Refresh error: " + compareLiteral(v.err)
	} else if v.viewerErr != "" {
		status = " Viewer: " + compareLiteral(v.viewerErr)
	} else if len(v.snapshot.files) == 0 && !initial {
		status = " Working tree matches HEAD · watching for changes"
	}
	chrome(1, status, base.Foreground(t.grayColor))
	for y := 0; y < h-3 && v.start+y < len(v.visual); y++ {
		segment := v.visual[v.start+y]
		index := segment.row
		r := v.rows[index]
		// The cursor outranks hover: the pointer must not hide the selection.
		style := base
		if index == v.cursor {
			style = t.cursorStyle
		} else if r.id == v.hover {
			style = t.selectionStyle
		}
		f.Buffer().SetStyle(catatui.NewRect(area.X, area.Y+uint16(y+2), area.Width, 1), style.Native())
		prefix := segment.prefix
		if index == v.cursor && !segment.continuation {
			prefix = "> " + strings.TrimPrefix(prefix, "  ")
		}
		color := t.whiteColor
		switch r.kind {
		case 'd':
			color = t.accentColor4
		case 'f':
			color = t.accentColor2
		case 'h':
			color = t.accentColor3
		case 'i':
			color = t.grayColor
		case 'l':
			l := v.snapshot.files[r.file].hunks[r.hunk].lines[r.line]
			color = t.grayColor
			if l.kind == '+' {
				color = t.greenColor
			}
			if l.kind == '-' {
				color = t.redColor
			}
			// Reuse the grapheme-safe, control-escaping text renderer.
			text := style.Foreground(color).Render(segment.text)
			put(y+2, style.Foreground(t.grayColor).Render(prefix)+text)
			continue
		}
		put(y+2, style.Foreground(t.grayColor).Render(prefix)+style.Foreground(color).Bold(r.kind == 'd' || r.kind == 'f').Render(segment.text))
	}
	wrap := "on"
	if v.noWrap {
		wrap = "off"
	}
	// Same shape as the bookmarks, tabs and Git list key lines. Keys are listed
	// by importance and dropped whole from the end when the row is too narrow.
	gray := base.Foreground(t.grayColor)
	footer, used := gray.Render(" Keys:"), len(" Keys:")
	for _, k := range [][2]string{{"F3", "view"}, {"[ ]", "prev/next hunk"}, {"space", "toggle"}, {"e", "expand all"}, {"c", "collapse all"}, {"w", "wrap:" + wrap}, {"Esc", "close"}} {
		cells := ansi.StringWidth(" " + k[0] + " - " + k[1])
		if used+cells > w {
			break
		}
		footer += base.Render(" "+k[0]+" ") + gray.Render("- "+k[1])
		used += cells
	}
	textAt(f, catatui.NewRect(area.X, area.Y+uint16(h-1), area.Width, 1), base.Width(w).Render(footer))
	viewport := max(1, h-3)
	state := widgets.NewScrollbarState(max(0, len(v.visual)-viewport) + 1).Position(v.start).ViewportContentLength(viewport)
	bar := widgets.NewScrollbar(widgets.ScrollbarVerticalRight).TrackStyle(base.Foreground(t.grayColor).Native()).ThumbStyle(base.Foreground(t.accentColor3).Native()).BeginSymbolNone().EndSymbolNone()
	catatui.RenderStatefulWidgetOn(f, bar, catatui.NewRect(area.X, area.Y+2, area.Width, uint16(viewport)), &state)
}
