package main

import (
	"fmt"
	"image/color"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Fiend3d/catatui"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
	"mc/internal/paint"
)

func compareTint(background, accent color.Color, percent uint32) color.Color {
	if background == nil {
		background = color.Black
	}
	br, bg, bb, _ := background.RGBA()
	ar, ag, ab, _ := accent.RGBA()
	return color.RGBA{uint8((br*(100-percent) + ar*percent) / 100 >> 8), uint8((bg*(100-percent) + ag*percent) / 100 >> 8), uint8((bb*(100-percent) + ab*percent) / 100 >> 8), 255}
}

// Source text is escaped before styling: embedded ANSI/OSC and other control
// characters cannot change the terminal or hide differences.
func compareLiteral(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) || (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069') {
			fmt.Fprintf(&b, "\\u%04X", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Clip by grapheme cell width before generating styled spans. Even a multi-MiB
// line only produces a viewport's worth of ANSI; wide glyphs are never split.
func compareLineText(line compareLine, marks []bool, offset, width int, base, strong paint.Style, ending bool) string {
	var b strings.Builder
	column, runeIndex := 0, 0
	graphemes := uniseg.NewGraphemes(line.text)
	for graphemes.Next() {
		cluster := graphemes.Str()
		n := utf8.RuneCountInString(cluster)
		highlight := false
		for i := runeIndex; i < runeIndex+n && i < len(marks); i++ {
			highlight = highlight || marks[i]
		}
		runeIndex += n
		text := compareLiteral(cluster)
		if cluster == "\t" {
			text = strings.Repeat(" ", 4-column%4)
		}
		w := ansi.StringWidth(text)
		if column+w > offset && column < offset+width {
			style := base
			if highlight {
				style = strong
			}
			if column < offset || column+w > offset+width {
				text = strings.Repeat(" ", max(0, min(column+w, offset+width)-max(column, offset)))
			}
			b.WriteString(style.Render(text))
		}
		column += w
		if column >= offset+width {
			return b.String()
		}
	}
	if ending {
		label := " [no newline]"
		switch line.ending {
		case "\n":
			label = " [LF]"
		case "\r\n":
			label = " [CRLF]"
		case "\r":
			label = " [CR]"
		}
		from, to := max(0, offset-column), min(len(label), offset+width-column)
		if to > from {
			b.WriteString(strong.Render(label[from:to]))
		}
	}
	return b.String()
}

func (m *model) drawCompare(f *catatui.Frame, area catatui.Rect) {
	c, t := &m.compare, m.theme
	w, h := int(area.Width), int(area.Height)
	base := t.baseStyle
	f.Buffer().SetStyle(area, base.Native())
	line := func(y int, s string, style paint.Style) {
		if y >= 0 && y < h {
			textAt(f, catatui.NewRect(area.X, area.Y+uint16(y), area.Width, 1), style.Width(w).Render(truncate(s, w)))
		}
	}
	status := "Files differ"
	if c.loading {
		status = "Comparing..."
	} else if c.result.err != nil {
		status = "Unable to compare"
	} else if c.result.equal {
		status = "Files are identical"
	} else if len(c.result.hunks) > 0 {
		status = fmt.Sprintf("Difference %d / %d", max(1, c.hunkAtStart()+1), len(c.result.hunks))
	}
	line(0, " Differences  ·  "+status, base.Bold(true).Foreground(t.accentColor3))
	leftWidth := (w - 1) / 2
	for side := 0; side < 2; side++ {
		x, width := 0, leftWidth
		if side == 1 {
			x = leftWidth + 1
			width = w - x
		}
		header := fmt.Sprintf(" %s  %s", []string{"LEFT", "RIGHT"}[side], compareLiteral(c.paths[side]))
		textAt(f, catatui.NewRect(area.X+uint16(x), area.Y+1, uint16(width), 1), base.Bold(true).Width(width).Render(truncate(header, width)))
	}
	note := c.result.note
	if c.result.err != nil {
		note = compareLiteral(c.result.err.Error())
	}
	if !c.loading && c.result.err == nil {
		note = fmt.Sprintf(" %d B / %d B", c.result.sizes[0], c.result.sizes[1]) + "  " + note
	}
	line(2, note, base.Foreground(t.grayColor))
	rows := max(0, h-4)
	c.start = min(max(0, c.start), max(0, len(c.result.rows)-1))
	for y := 0; y < rows; y++ {
		index := c.start + y
		if index >= len(c.result.rows) {
			break
		}
		row := c.result.rows[index]
		for side, number := range []int{row.left, row.right} {
			x, width := 0, leftWidth
			accent := t.redColor
			if side == 1 {
				x = leftWidth + 1
				width = w - x
				accent = t.greenColor
			}
			style := base.Foreground(t.grayColor)
			if row.changed {
				style = base.Background(compareTint(base.GetBackground(), accent, 14))
			}
			strong := style.Background(compareTint(base.GetBackground(), accent, 35)).Bold(true)
			a := catatui.NewRect(area.X+uint16(x), area.Y+uint16(y+3), uint16(width), 1)
			f.Buffer().SetStyle(a, style.Native())
			if number == 0 {
				continue
			}
			digits := max(3, len(fmt.Sprint(len(c.result.lines[side]))))
			marker := " "
			if row.changed {
				marker = []string{"−", "+"}[side]
			}
			prefix := fmt.Sprintf(" %*d %s ", digits, number, marker)
			marks := row.leftMarks
			if side == 1 {
				marks = row.rightMarks
			}
			text := compareLineText(c.result.lines[side][number-1], marks, c.horizontal, max(0, width-digits-5), style, strong, row.changed)
			gutter := style.Foreground(t.grayColor)
			if row.changed {
				gutter = style.Foreground(accent)
			}
			textAt(f, a, gutter.Render(prefix)+text)
		}
	}
	for y := 1; y < h-1; y++ {
		textAt(f, catatui.NewRect(area.X+uint16(leftWidth), area.Y+uint16(y), 1, 1), base.Foreground(t.grayColor).Render("│"))
	}
	footer := " n/p difference  ↑↓ scroll  ←→ pan  F5 reload  w tasks  Esc close"
	if w < 75 {
		footer = " n/p diff  ↑↓ ←→  F5 reload  Esc close"
	}
	line(h-1, footer, base.Foreground(t.grayColor))
}
