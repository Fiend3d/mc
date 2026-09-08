// Package paint composes styled text for mc's text-heavy views. The resulting
// spans are rendered into catatui buffers; they are never written to the terminal.
package paint

import (
	"fmt"
	"github.com/Fiend3d/catatui"
	"github.com/charmbracelet/x/ansi"
	"image/color"
	"regexp"
	"strconv"
	"strings"
)

type Position float64

const (
	Left   Position = 0
	Center Position = .5
	Right  Position = 1
)

type NoColor struct{}

type indexedColor struct {
	color.Color
	index uint8
}

func (NoColor) RGBA() (uint32, uint32, uint32, uint32) { return 0, 0, 0, 0 }
func Color(s string) color.Color {
	if strings.HasPrefix(s, "#") {
		n, _ := strconv.ParseUint(s[1:], 16, 32)
		return color.RGBA{uint8(n >> 16), uint8(n >> 8), uint8(n), 255}
	}
	n, _ := strconv.Atoi(s)
	if n >= 232 {
		v := uint8(8 + (n-232)*10)
		return color.RGBA{v, v, v, 255}
	}
	palette := []uint32{0x000000, 0xcc3333, 0x33aa55, 0xccbb44, 0x4488cc, 0xaa55bb, 0x33aaaa, 0xcccccc, 0x666666, 0xff5555, 0x55ff55, 0xffff55, 0x5555ff, 0xff55ff, 0x55ffff, 0xffffff}
	if n < 0 || n >= len(palette) {
		n = 7
	}
	v := palette[n]
	return indexedColor{color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, uint8(n)}
}

var (
	BrightBlack = Color("8")
	Green       = Color("2")
	Red         = Color("1")
	Cyan        = Color("6")
	BrightCyan  = Color("14")
	BrightBlue  = Color("12")
	Yellow      = Color("3")
	BrightRed   = Color("9")
)

func LightDark(dark bool) func(color.Color, color.Color) color.Color {
	return func(a, b color.Color) color.Color {
		if dark {
			return b
		}
		return a
	}
}

type Style struct {
	fg, bg        color.Color
	bold, reverse bool
	width         int
	align         Position
}

func NewStyle() Style                          { return Style{} }
func (s Style) Foreground(c color.Color) Style { s.fg = c; return s }
func (s Style) Background(c color.Color) Style { s.bg = c; return s }
func (s Style) GetForeground() color.Color     { return s.fg }
func (s Style) GetBackground() color.Color     { return s.bg }
func (s Style) Bold(b bool) Style              { s.bold = b; return s }
func (s Style) Reverse(b bool) Style           { s.reverse = b; return s }
func (s Style) Inline(bool) Style              { return s }
func (s Style) Width(w int) Style              { s.width = max(0, w); return s }
func (s Style) Align(p Position) Style         { s.align = p; return s }
func (s Style) Native() catatui.Style {
	t := catatui.NewStyle()
	if s.fg != nil {
		t = t.Fg(nativeColor(s.fg))
	}
	if s.bg != nil {
		t = t.Bg(nativeColor(s.bg))
	}
	if s.bold {
		t = t.AddModifier(catatui.ModifierBold)
	}
	if s.reverse {
		t = t.AddModifier(catatui.ModifierReversed)
	}
	return t
}
func nativeColor(c color.Color) catatui.Color {
	if c, ok := c.(indexedColor); ok {
		return catatui.Indexed(c.index)
	}
	r, g, b, a := c.RGBA()
	if a == 0 {
		return catatui.ColorReset
	}
	return catatui.Rgb(uint8(r>>8), uint8(g>>8), uint8(b>>8))
}
func (s Style) Render(parts ...string) string {
	text := strings.Join(parts, " ")
	lines := strings.Split(text, "\n")
	prefix := "\x1b[0m"
	if s.bold {
		prefix += "\x1b[1m"
	}
	if s.reverse {
		prefix += "\x1b[7m"
	}
	for i, c := range []color.Color{s.fg, s.bg} {
		if c != nil {
			if c, ok := c.(indexedColor); ok {
				prefix += fmt.Sprintf("\x1b[%d;5;%dm", 38+i*10, c.index)
				continue
			}
			r, g, b, a := c.RGBA()
			if a != 0 {
				prefix += fmt.Sprintf("\x1b[%d;2;%d;%d;%dm", 38+i*10, r>>8, g>>8, b>>8)
			}
		}
	}
	for i, line := range lines {
		line = PlaceHorizontal(s.width, s.align, line)
		lines[i] = prefix + strings.ReplaceAll(line, "\x1b[0m", prefix) + "\x1b[0m"
	}
	return strings.Join(lines, "\n")
}
func Width(s string) int {
	w := 0
	for _, line := range strings.Split(s, "\n") {
		w = max(w, catatui.StringWidth(ansi.Strip(line)))
	}
	return w
}

// Truncate uses the same grapheme widths as catatui while retaining SGR spans.
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if Width(s) <= width {
		return s
	}
	var out strings.Builder
	used, pos := 0, 0
	appendText := func(text string) bool {
		for g := range catatui.SegmentGraphemes(text) {
			if used+int(g.Width) > width-1 {
				return false
			}
			out.WriteString(g.Symbol)
			used += int(g.Width)
		}
		return true
	}
	for _, loc := range sgr.FindAllStringIndex(s, -1) {
		if !appendText(s[pos:loc[0]]) {
			return out.String() + "…\x1b[0m"
		}
		out.WriteString(s[loc[0]:loc[1]])
		pos = loc[1]
	}
	appendText(s[pos:])
	return out.String() + "…\x1b[0m"
}
func PlaceHorizontal(w int, p Position, s string) string {
	n := max(0, w-Width(s))
	left := int(float64(n) * float64(p))
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", n-left)
}

var sgr = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// Lines decodes our SGR style annotations into catatui's native text spans.
func Lines(s string) []catatui.Line {
	var lines []catatui.Line
	for _, line := range strings.Split(s, "\n") {
		var spans []catatui.Span
		style := catatui.NewStyle()
		pos := 0
		for _, loc := range sgr.FindAllStringSubmatchIndex(line, -1) {
			if loc[0] > pos {
				spans = append(spans, catatui.NewStyledSpan(line[pos:loc[0]], style))
			}
			codes := strings.Split(line[loc[2]:loc[3]], ";")
			for i := 0; i < len(codes); i++ {
				n, _ := strconv.Atoi(codes[i])
				switch n {
				case 0:
					style = catatui.NewStyle()
				case 1:
					style = style.AddModifier(catatui.ModifierBold)
				case 7:
					style = style.AddModifier(catatui.ModifierReversed)
				case 38, 48:
					if i+2 < len(codes) && codes[i+1] == "5" {
						index, _ := strconv.Atoi(codes[i+2])
						c := catatui.Indexed(uint8(index))
						if n == 38 {
							style = style.Fg(c)
						} else {
							style = style.Bg(c)
						}
						i += 2
						continue
					}
					if i+4 < len(codes) && codes[i+1] == "2" {
						r, _ := strconv.Atoi(codes[i+2])
						g, _ := strconv.Atoi(codes[i+3])
						b, _ := strconv.Atoi(codes[i+4])
						c := catatui.Rgb(uint8(r), uint8(g), uint8(b))
						if n == 38 {
							style = style.Fg(c)
						} else {
							style = style.Bg(c)
						}
						i += 4
					}
				}
			}
			pos = loc[1]
		}
		if pos < len(line) {
			spans = append(spans, catatui.NewStyledSpan(line[pos:], style))
		}
		lines = append(lines, catatui.NewLine(spans...))
	}
	return lines
}
