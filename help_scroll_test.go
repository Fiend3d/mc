package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Fiend3d/catatui"
	"github.com/Fiend3d/catatui/term"
	"mc/internal/event"
)

// cellBackground names a cell's background colour so backgrounds can be
// compared across a screen.
func cellBackground(c catatui.Cell) string {
	r, g, b, ok := c.Bg.RGB()
	if !ok {
		return "default"
	}
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// helpMouse feeds a raw terminal mouse event through the real input pipeline.
func helpMouse(t *testing.T, m *model, kind term.MouseKind, x, y int) {
	t.Helper()
	msg := m.inputEvent(term.Event{
		Kind: term.EventMouse, MouseKind: kind,
		Button: term.MouseButtonLeft, X: uint16(x), Y: uint16(y),
	})
	if msg != nil {
		m.Update(msg)
	}
}

func openHelp(t *testing.T, w, h int) *model {
	t.Helper()
	m := testModel(t, t.TempDir())
	m.screenWidth, m.screenHeight = w, h
	m.dimensions()
	keyEvent(m, "f1")
	helpScrollColumn(t, m, w, h) // one render measures the view
	return m
}

const (
	helpScrollThumb = "█"
	helpScrollTrack = "║"
)

// helpScrollColumn renders the help view and returns its right-hand column,
// one rune per screen row.
func helpScrollColumn(t *testing.T, m *model, w, h int) string {
	t.Helper()
	backend := catatui.NewTestBackend(uint16(w), uint16(h))
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	var col strings.Builder
	for _, line := range strings.Split(backend.Buffer().String(), "\n") {
		runes := []rune(line)
		if len(runes) == 0 {
			continue
		}
		col.WriteRune(runes[len(runes)-1])
	}
	return col.String()
}

func TestHelpScrollbarTracksTheViewport(t *testing.T) {
	m := testModel(t, t.TempDir())
	m.screenWidth, m.screenHeight = 70, 20
	m.dimensions()
	keyEvent(m, "f1")

	col := helpScrollColumn(t, m, 70, 20)
	if m.helpLines <= m.helpViewport() {
		t.Fatalf("this test needs help longer than the viewport: %d lines", m.helpLines)
	}
	if !strings.HasPrefix(col, helpScrollThumb) {
		t.Fatalf("thumb is not at the top at offset 0: %q", col)
	}
	if !strings.Contains(col, helpScrollTrack) {
		t.Fatalf("no track drawn: %q", col)
	}
	// The filter prompt on the last row keeps its full width.
	if last := []rune(col)[len([]rune(col))-1]; string(last) == helpScrollThumb || string(last) == helpScrollTrack {
		t.Fatalf("scrollbar overran the filter row: %q", col)
	}

	for i := 0; i < 500; i++ {
		keyEvent(m, "j")
	}
	if m.help != m.maxHelpScroll() {
		t.Fatalf("scroll ran past the end: help=%d max=%d", m.help, m.maxHelpScroll())
	}
	col = helpScrollColumn(t, m, 70, 20)
	runes := []rune(col)
	if string(runes[0]) != helpScrollTrack {
		t.Fatalf("thumb did not leave the top after scrolling: %q", col)
	}
	// Row 18 is the last documentation row; 19 is the filter prompt.
	if string(runes[18]) != helpScrollThumb {
		t.Fatalf("thumb is not at the bottom at max scroll: %q", col)
	}
}

func TestHelpScrollSurvivesFirstKeyBeforeAnyRender(t *testing.T) {
	m := testModel(t, t.TempDir())
	m.screenWidth, m.screenHeight = 70, 20
	m.dimensions()
	keyEvent(m, "f1")
	// Nothing has been drawn, so there is no measured length to clamp against;
	// the keypress must still move rather than being swallowed.
	if m.helpLines != 0 {
		t.Fatalf("precondition: expected an unmeasured help view, got %d", m.helpLines)
	}
	keyEvent(m, "j")
	if m.help != 1 {
		t.Fatalf("first keypress was swallowed: help=%d", m.help)
	}
	helpScrollColumn(t, m, 70, 20)
	if m.help != 1 || m.helpLines == 0 {
		t.Fatalf("render did not settle the offset: help=%d lines=%d", m.help, m.helpLines)
	}
}

func TestHelpFilterShrinksScrollRange(t *testing.T) {
	m := testModel(t, t.TempDir())
	m.screenWidth, m.screenHeight = 70, 20
	m.dimensions()
	keyEvent(m, "f1")
	helpScrollColumn(t, m, 70, 20)

	full := m.helpLines
	for i := 0; i < 500; i++ {
		keyEvent(m, "j")
	}
	bottom := m.help
	if bottom == 0 {
		t.Fatal("precondition: unfiltered help should scroll")
	}

	// Filtering removes most of the documentation, so the offset must follow it
	// down instead of leaving the viewport parked past the end.
	m.helpFilter = "Quit"
	col := helpScrollColumn(t, m, 70, 20)
	if m.helpLines >= full {
		t.Fatalf("filter did not shorten the help: %d of %d", m.helpLines, full)
	}
	if m.help > m.maxHelpScroll() {
		t.Fatalf("offset left past the end: help=%d max=%d", m.help, m.maxHelpScroll())
	}
	if m.helpLines <= m.helpViewport() && strings.Contains(col, helpScrollTrack) {
		t.Fatalf("scrollbar drawn for filtered help that fits: %q", col)
	}
}

// helpRender returns the drawn help screen as lines.
func helpRender(t *testing.T, m *model, w, h int) []string {
	t.Helper()
	backend := catatui.NewTestBackend(uint16(w), uint16(h))
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	return strings.Split(backend.Buffer().String(), "\n")
}

func TestHelpEntriesShareAKeyColumn(t *testing.T) {
	const w, h = 78, 24
	m := openHelp(t, w, h)
	lines := helpRender(t, m, w, h)

	find := func(needle string) int {
		for i, line := range lines {
			if strings.Contains(line, needle) {
				return i
			}
		}
		t.Fatalf("%q not found in:\n%s", needle, strings.Join(lines, "\n"))
		return -1
	}

	first := find("Quit and return the current directory.")
	second := find("Quit without returning a directory.")
	// Compact layout: consecutive shortcuts sit on consecutive rows.
	if second != first+1 {
		t.Fatalf("blank line between entries: rows %d and %d", first, second)
	}
	// Aligned layout: both descriptions start in the same column.
	c1 := strings.Index(lines[first], "Quit and return")
	c2 := strings.Index(lines[second], "Quit without")
	if c1 != c2 {
		t.Fatalf("descriptions start at columns %d and %d", c1, c2)
	}
	// The key column really is a column: a wide key keeps the same alignment.
	wide := find("Toggle the selection and move down.")
	if got := strings.Index(lines[wide], "Toggle the selection"); got != c1 {
		t.Fatalf("wide shortcut broke the column: %d vs %d", got, c1)
	}

	// A wrapped description hangs under the description column rather than
	// falling back to the left margin.
	head := find("Select everything up to the clicked item")
	tail := lines[head+1]
	if strings.TrimSpace(tail) == "" {
		t.Fatalf("expected a wrapped continuation after row %d", head)
	}
	if got := len(tail) - len(strings.TrimLeft(tail, " ")); got != c1 {
		t.Fatalf("continuation indented to %d, want the description column %d: %q", got, c1, tail)
	}
}

func TestHelpTopicBlurbSitsUnderTheHeader(t *testing.T) {
	const w, h = 78, 24
	m := openHelp(t, w, h)
	// Filter to the Go topic so the whole section fits on one screen.
	m.helpFilter = "Enter Path mode"
	lines := helpRender(t, m, w, h)

	header, blurb, entry := -1, -1, -1
	for i, line := range lines {
		switch {
		case strings.Contains(line, "Go Mode"):
			header = i
		case strings.Contains(line, "Go mode is just a menu."):
			blurb = i
		case strings.Contains(line, "Enter Path mode."):
			entry = i
		}
	}
	if header < 0 || blurb < 0 || entry < 0 {
		t.Fatalf("missing rows: header=%d blurb=%d entry=%d", header, blurb, entry)
	}
	if blurb != header+1 {
		t.Fatalf("blurb is not directly under the header: %d vs %d", blurb, header)
	}
	// The description belongs to its header, so the two share a left margin.
	// Indenting it to the key column made prose line up with the shortcuts.
	headerCol := len(lines[header]) - len(strings.TrimLeft(lines[header], " "))
	blurbCol := len(lines[blurb]) - len(strings.TrimLeft(lines[blurb], " "))
	if blurbCol != headerCol {
		t.Fatalf("blurb starts at column %d, header at %d", blurbCol, headerCol)
	}
	keyCol := strings.Index(lines[entry], "g ")
	if blurbCol >= keyCol {
		t.Fatalf("blurb indented into the shortcut list: %d, keys start at %d", blurbCol, keyCol)
	}
	if strings.TrimSpace(lines[blurb+1]) != "" {
		t.Fatalf("expected a blank line between the blurb and the shortcuts: %q", lines[blurb+1])
	}
}

func TestHelpBlurbSurvivesAFilterThatMatchesOnlyIt(t *testing.T) {
	const w, h = 78, 24
	m := openHelp(t, w, h)
	// "menu" appears only in the Go topic's description, in no shortcut.
	m.helpFilter = "menu"
	lines := helpRender(t, m, w, h)

	joined := strings.Join(lines, " ")
	if !strings.Contains(joined, "Go Mode") || !strings.Contains(joined, "Go mode is just a menu.") {
		t.Fatalf("a filter matching only the description dropped the topic:\n%s", strings.Join(lines, "|"))
	}
}

func TestHelpOrphanedNoteReadsAsProse(t *testing.T) {
	const w, h = 78, 24
	m := openHelp(t, w, h)
	// This note elaborates on Ctrl+Left/Right, which this filter removes.
	m.helpFilter = "An empty pane keeps"
	lines := helpRender(t, m, w, h)

	note := -1
	for i, line := range lines {
		if strings.Contains(line, "An empty pane keeps") {
			note = i
		}
	}
	if note < 0 {
		t.Fatal("the note was filtered out entirely")
	}
	// With nothing above it to hang from, a deep indent would look like a stray
	// continuation line.
	if got := len(lines[note]) - len(strings.TrimLeft(lines[note], " ")); got > len(helpIndent) {
		t.Fatalf("orphaned note indented to %d, want at most %d: %q", got, len(helpIndent), lines[note])
	}
}

func TestHelpTitleCentresOverTheWholeWidth(t *testing.T) {
	const w, h = 78, 24
	m := openHelp(t, w, h)
	lines := helpRender(t, m, w, h)

	// Measure the text region only: the scrollbar glyph sits in the last column
	// and would otherwise count as part of the banner.
	row := lines[0]
	if len(row) > w-2 {
		row = row[:w-2]
	}
	lead := strings.Index(row, "Modal Commander")
	if lead < 0 {
		t.Fatalf("title row missing: %q", lines[0])
	}
	text := strings.TrimSpace(row)
	// Centring within the text width alone would push the banner left by the
	// columns reserved for the scrollbar.
	want := (w - len(text)) / 2
	if lead != want {
		t.Fatalf("title starts at %d, want %d (width %d, text %d)", lead, want, w, len(text))
	}
}

func TestHelpFooterSharesTheHeaderMargin(t *testing.T) {
	const w, h = 78, 24
	m := openHelp(t, w, h)
	lines := helpRender(t, m, w, h)

	header := -1
	for i, line := range lines {
		if strings.Contains(line, "Normal Mode") {
			header = i
		}
	}
	if header < 0 {
		t.Fatal("no topic header found")
	}
	footer := lines[len(lines)-1]
	if !strings.Contains(footer, "Press ") {
		t.Fatalf("last row is not the footer: %q", footer)
	}
	headerCol := len(lines[header]) - len(strings.TrimLeft(lines[header], " "))
	footerCol := len(footer) - len(strings.TrimLeft(footer, " "))
	if footerCol != headerCol {
		t.Fatalf("footer starts at column %d, headers at %d", footerCol, headerCol)
	}
}

func TestHelpScrollbarDragsWithTheMouse(t *testing.T) {
	const w, h = 78, 24
	m := openHelp(t, w, h)
	col, track := m.helpScrollbarColumn(), m.helpViewport()
	if m.maxHelpScroll() == 0 {
		t.Fatal("precondition: help should overflow at this size")
	}

	helpMouse(t, m, term.MouseDown, col, 0)
	if !m.helpDragging {
		t.Fatal("pressing on the scrollbar did not start a drag")
	}
	if m.help != 0 {
		t.Fatalf("pressing at the top should sit at offset 0, got %d", m.help)
	}

	helpMouse(t, m, term.MouseDrag, col, track/2)
	middle := m.help
	if middle <= 0 || middle >= m.maxHelpScroll() {
		t.Fatalf("dragging to the middle gave %d, want between 0 and %d", middle, m.maxHelpScroll())
	}

	helpMouse(t, m, term.MouseDrag, col, track-1)
	if m.help != m.maxHelpScroll() {
		t.Fatalf("dragging to the bottom gave %d, want %d", m.help, m.maxHelpScroll())
	}

	helpMouse(t, m, term.MouseUp, col, track-1)
	if m.helpDragging {
		t.Fatal("releasing did not end the drag")
	}
	// With the button up, moving the pointer must not scroll.
	helpMouse(t, m, term.MouseDrag, col, 0)
	if m.help != m.maxHelpScroll() {
		t.Fatalf("pointer moved the view after release: %d", m.help)
	}
}

func TestHelpClickOffTheScrollbarDoesNotDrag(t *testing.T) {
	const w, h = 78, 24
	m := openHelp(t, w, h)
	col := m.helpScrollbarColumn()

	helpMouse(t, m, term.MouseDown, col-2, 5)
	if m.helpDragging {
		t.Fatal("a click on the text started a drag")
	}
	if m.help != 0 {
		t.Fatalf("a click on the text scrolled the view to %d", m.help)
	}
	// A press below the track, on the filter row, is not the scrollbar either.
	helpMouse(t, m, term.MouseDown, col, m.helpViewport())
	if m.helpDragging {
		t.Fatal("a click on the filter row started a drag")
	}
}

func TestHelpWheelStopsAtTheEnd(t *testing.T) {
	const w, h = 78, 24
	m := openHelp(t, w, h)
	for i := 0; i < 200; i++ {
		m.handleWheel(3)
	}
	if m.help != m.maxHelpScroll() {
		t.Fatalf("wheel ran past the end: help=%d max=%d", m.help, m.maxHelpScroll())
	}
	for i := 0; i < 200; i++ {
		m.handleWheel(-3)
	}
	if m.help != 0 {
		t.Fatalf("wheel did not return to the top: %d", m.help)
	}
}

func TestHelpKeepsOneBackgroundAcrossResizes(t *testing.T) {
	m := testModel(t, t.TempDir())
	m.screenWidth, m.screenHeight = 100, 30
	m.dimensions()
	keyEvent(m, "f1")

	backend := catatui.NewTestBackend(100, 30)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		keyEvent(m, "j") // scroll away from the top so the thumb is mid-track
	}

	// Resizing changes the thumb's length and position. The scrollbar lane, its
	// gutter and the body must stay one colour throughout, or the column
	// appears to change colour as the window is dragged.
	// Every step must be a real size change: resizing the backend to its
	// current size blanks it while the terminal keeps diffing against the old
	// frame, so unchanged cells are legitimately never reflushed.
	for _, size := range [][2]int{{60, 18}, {120, 40}, {45, 10}, {78, 24}, {100, 30}} {
		w, h := size[0], size[1]
		backend.Resize(uint16(w), uint16(h))
		m.Update(event.WindowSizeMsg{Width: w, Height: h})
		if err := terminal.Draw(m.draw); err != nil {
			t.Fatal(err)
		}
		seen := map[string]int{}
		for _, c := range backend.Buffer().Content {
			seen[cellBackground(c)]++
		}
		if len(seen) != 1 {
			t.Fatalf("%dx%d help screen uses %d backgrounds: %v", w, h, len(seen), seen)
		}
	}
}

func TestHelpScrollbarThumbAndTrackShareABackground(t *testing.T) {
	const w, h = 78, 24
	m := openHelp(t, w, h)
	m.help = m.maxHelpScroll() / 2 // thumb somewhere in the middle
	backend := catatui.NewTestBackend(w, h)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}

	col := w - 1
	var thumb, track string
	for y := 0; y < m.helpViewport(); y++ {
		c := backend.Buffer().Content[y*w+col]
		switch c.GetSymbol() {
		case helpScrollThumb:
			thumb = cellBackground(c)
		case helpScrollTrack:
			track = cellBackground(c)
		}
	}
	if thumb == "" || track == "" {
		t.Fatalf("expected both a thumb and a track, got thumb=%q track=%q", thumb, track)
	}
	if thumb != track {
		t.Fatalf("thumb background %s differs from track %s, so the column changes colour as it moves", thumb, track)
	}
}

func TestHelpWithoutOverflowHasNoScrollbar(t *testing.T) {
	m := testModel(t, t.TempDir())
	m.screenWidth, m.screenHeight = 70, 20
	m.dimensions()
	keyEvent(m, "f1")
	// A screen taller than the documentation leaves nothing to scroll.
	m.screenHeight = m.helpLines + 4
	m.dimensions()

	col := helpScrollColumn(t, m, 70, m.screenHeight)
	if strings.Contains(col, helpScrollThumb) || strings.Contains(col, helpScrollTrack) {
		t.Fatalf("scrollbar drawn for help that fits: %q", col)
	}
	if m.maxHelpScroll() != 0 {
		t.Fatalf("expected no scroll room, got %d", m.maxHelpScroll())
	}
}
