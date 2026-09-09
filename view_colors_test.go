package main

import (
	"github.com/Fiend3d/catatui"
	"mc/internal/paint"
	"strings"
	"testing"
)

func TestNativeThemeFileColors(t *testing.T) {
	for _, preset := range themeList {
		t.Run(preset.name, func(t *testing.T) {
			m := testModel(t, t.TempDir())
			m.theme = newTheme(preset.name)
			m.getPage().items = []item{
				&filepathItem{name: "folder", isDir: true},
				&filepathItem{name: "tool.EXE"},
				&filepathItem{name: "notes.txt"},
			}
			backend := catatui.NewTestBackend(100, 24)
			terminal, _ := catatui.NewTerminal(backend)
			if err := terminal.Draw(m.draw); err != nil {
				t.Fatal(err)
			}
			b := backend.Buffer()
			want := []catatui.Color{
				m.theme.baseStyle.Foreground(m.theme.accentColor4).Native().GetFg(),
				m.theme.baseStyle.Foreground(m.theme.greenColor).Native().GetFg(),
				m.theme.baseStyle.Foreground(m.theme.whiteColor).Native().GetFg(),
			}
			for row, fg := range want {
				if got := b.CellAt(3, uint16(row+2)).Fg; got != fg {
					t.Fatalf("row %d foreground = %v, want %v", row, got, fg)
				}
			}
			lines := strings.Split(b.String(), "\n")
			if !strings.Contains(lines[len(lines)-1], "folder") {
				t.Fatal("bottom row does not contain the idle footer")
			}
		})
	}
}

func TestNativeSelectionAndClipboardMarkers(t *testing.T) {
	m := testModel(t, t.TempDir())
	m.getPage().items = []item{
		&filepathItem{name: "copied.txt", selected: true, action: itemActionCopy},
		&filepathItem{name: "cut.txt", action: itemActionCut},
		&filepathItem{name: "plain.txt"},
	}
	m.getTab().getPageSettings().cursor = 1
	backend := catatui.NewTestBackend(100, 24)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	b := backend.Buffer()
	copyColor := m.theme.baseStyle.Foreground(m.theme.accentColor4).Native().GetFg()
	cutColor := m.theme.baseStyle.Foreground(m.theme.accentColor2).Native().GetFg()
	if cell := b.CellAt(0, 2); cell.GetSymbol() != "┃" || cell.Fg != copyColor {
		t.Fatalf("copy marker = %q %v", cell.GetSymbol(), cell.Fg)
	}
	if cell := b.CellAt(0, 3); cell.GetSymbol() != "┃" || cell.Fg != cutColor {
		t.Fatalf("cut marker = %q %v", cell.GetSymbol(), cell.Fg)
	}
	if cell := b.CellAt(2, 2); cell.GetSymbol() != "┃" {
		t.Fatalf("selection marker = %q, want line", cell.GetSymbol())
	} else if want := m.theme.baseStyle.Foreground(m.theme.whiteColor).Native().GetFg(); cell.Fg != want {
		t.Fatalf("selection marker color = %v, want %v", cell.Fg, want)
	}
	selectedBackground := m.theme.selectionStyle.Native().GetBg()
	if got := b.CellAt(4, 2).Bg; got != selectedBackground {
		t.Fatalf("selected background = %v, want %v", got, selectedBackground)
	}
	if got, want := b.CellAt(4, 3).Bg, m.theme.cursorStyle.Native().GetBg(); got != want {
		t.Fatalf("cursor background = %v, want %v", got, want)
	}
	m.hoverPane, m.hoverIndex = 0, 0
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	if got, want := b.CellAt(4, 2).Bg, m.theme.cursorStyle.Native().GetBg(); got != want {
		t.Fatalf("selected hover background = %v, want %v", got, want)
	}
	m.hoverIndex = 2
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	if got, want := b.CellAt(4, 4).Bg, m.theme.selectionStyle.Native().GetBg(); got != want {
		t.Fatalf("plain hover background = %v, want %v", got, want)
	}
}

func TestNativeSearchHoverHighlight(t *testing.T) {
	m := testModel(t, t.TempDir())
	m.mode = searchMode
	m.search = newSearch(m)
	m.search.focus = 2
	m.search.items = []searchItem{{path: "first.txt"}, {path: "second.txt"}}
	m.search.cursor = 0
	m.hoverSearchIndex = 1
	backend := catatui.NewTestBackend(100, 24)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	if got, want := backend.Buffer().CellAt(4, 4).Bg, m.theme.selectionStyle.Native().GetBg(); got != want {
		t.Fatalf("search hover background = %v, want %v", got, want)
	}
	if got, want := backend.Buffer().CellAt(4, 3).Bg, m.theme.cursorStyle.Native().GetBg(); got != want {
		t.Fatalf("search cursor background = %v, want %v", got, want)
	}
}

func TestNativeTabAndBreadcrumbHover(t *testing.T) {
	m := testModel(t, t.TempDir())
	m.getTab().dir = `C:\one\two`
	m.panes[0].tabs = append(m.panes[0].tabs, newTab(`C:\other`, &page{}))
	m.hoverTabPane, m.hoverTabIndex = 0, 1
	m.hoverPathPane, m.hoverPathX = 0, 4
	backend := catatui.NewTestBackend(100, 24)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	b := backend.Buffer()
	secondTabX := uint16(paint.Width(paneTabLabels(m.panes[0])[0]) + 1)
	if got, want := b.CellAt(secondTabX, 0).Bg, m.theme.selectionStyle.Native().GetBg(); got != want {
		t.Fatalf("tab hover background = %v, want %v", got, want)
	}
	if got, want := b.CellAt(4, 1).Fg, m.theme.emptyStyle.Foreground(m.theme.whiteColor).Native().GetFg(); got != want {
		t.Fatalf("breadcrumb hover color = %v, want %v", got, want)
	}
	if got, want := b.CellAt(8, 1).Fg, m.theme.emptyStyle.Foreground(m.theme.accentColor5).Native().GetFg(); got != want {
		t.Fatalf("non-hovered breadcrumb color = %v, want %v", got, want)
	}
}
