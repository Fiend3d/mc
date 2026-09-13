package main

import (
	"errors"
	"slices"
	"strings"

	"github.com/Fiend3d/catatui/term"
	"mc/internal/event"
	"mc/internal/paint"
	"path/filepath"
	"testing"
)

func selectionModel(t *testing.T) *model {
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		touch(t, filepath.Join(dir, name))
	}
	m := testModel(t, dir, dir)
	applyEffect(m, m.readTab(m.getTab()))
	m.sort(alphabeticSort, false)
	return m
}

func assertSelection(t *testing.T, m *model, want string) {
	t.Helper()
	got := ""
	for _, it := range m.getPage().getItems() {
		if it.isSelected() {
			got += it.getName()
		}
	}
	if got != want {
		t.Fatalf("selection = %q, want %q", got, want)
	}
}

func TestRangeSelection(t *testing.T) {
	m := selectionModel(t)
	m.getPage().items[4].setSelected(true)
	keyEvent(m, "down")
	keyEvent(m, "shift+down")
	assertSelection(t, m, "bce")
	keyEvent(m, "shift+down")
	assertSelection(t, m, "bcde")
	keyEvent(m, "shift+up")
	assertSelection(t, m, "bce")
	keyEvent(m, "shift+home")
	assertSelection(t, m, "abe")
	keyEvent(m, "shift+end")
	assertSelection(t, m, "bcde")
	keyEvent(m, "home")
	keyEvent(m, "shift+down")
	assertSelection(t, m, "abcde")
	keyEvent(m, "ctrl+d")
	keyEvent(m, "shift+up")
	assertSelection(t, m, "ab")
	keyEvent(m, "shift+up")
	assertSelection(t, m, "ab")
}

func TestRangeInputAndMouse(t *testing.T) {
	m := selectionModel(t)
	for key, want := range map[term.KeyCode]string{term.KeyUp: "shift+up", term.KeyDown: "shift+down", term.KeyHome: "shift+home", term.KeyEnd: "shift+end", term.KeyBackTab: "shift+tab"} {
		msg := m.inputEvent(term.Event{Kind: term.EventKey, Key: key, Mods: term.ModShift}).(event.KeyMsg)
		if msg.Name != want {
			t.Fatalf("key = %q, want %q", msg.Name, want)
		}
	}
	m.Update(m.inputEvent(term.Event{Kind: term.EventKey, Key: term.KeyBackTab, Mods: term.ModShift}))
	if m.mode != normalMode {
		t.Fatal("Shift+Tab still enters Jump")
	}
	m.Update(m.inputEvent(term.Event{Kind: term.EventKey, Key: term.KeyRune, Rune: 'j', Mods: term.ModCtrl}))
	if m.mode != jumpMode {
		t.Fatal("Ctrl+J did not enter Jump")
	}
	m.Update(m.inputEvent(term.Event{Kind: term.EventKey, Key: term.KeyDown, Mods: term.ModShift}))
	assertSelection(t, m, "ab")
	for _, mods := range []term.Modifiers{term.ModShift, term.ModCtrl, term.ModCtrl} {
		m.Update(m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseDown, Button: term.MouseButtonLeft, X: 2, Y: 5, Mods: mods}))
	}
	assertSelection(t, m, "abcd")
	if m.click.doubleClick {
		t.Fatal("range click retained double-click state")
	}
	keyEvent(m, "esc")
	keyEvent(m, "tab")
	applyEffect(m, m.readTab(m.getTab()))
	m.sort(alphabeticSort, false)
	keyEvent(m, "end")
	// Clicking into the left pane anchors at its own cursor (d).
	m.Update(m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseDown, Button: term.MouseButtonLeft, X: 2, Y: 6, Mods: term.ModCtrl}))
	if m.activePane != 0 {
		t.Fatal("wrong pane")
	}
	assertSelection(t, m, "abcde")
	keyEvent(m, "tab")
	assertSelection(t, m, "")
}

func TestRangeListingChanges(t *testing.T) {
	m := selectionModel(t)
	keyEvent(m, "shift+down")
	applyEffect(m, m.readTab(m.getTab()))
	if m.getPage().selectionRange != nil {
		t.Fatal("refresh retained anchor")
	}
	keyEvent(m, "shift+down")
	m.sort(alphabeticSort, true)
	if m.getPage().selectionRange != nil {
		t.Fatal("sort retained anchor")
	}
	m.getTab().filterText = []string{"c"}
	m.getTab().filter()
	keyEvent(m, "shift+end")
	assertSelection(t, m, "c")
	m.getTab().filterText = []string{"missing"}
	m.getTab().filter()
	keyEvent(m, "shift+home")
	assertSelection(t, m, "")
	m.getTab().filterText = nil
	applyEffect(m, m.readTab(m.getTab()))
	if m.getPage().selectionRange != nil {
		t.Fatal("refresh retained anchor")
	}
}

func TestMouseHoverDoesNotMoveCursorOrFocus(t *testing.T) {
	m := selectionModel(t)
	applyEffect(m, m.readTab(m.panes[1].tabs[0]))
	msg := m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseMove, X: 2, Y: 4})
	hover, ok := msg.(event.MouseHoverMsg)
	if !ok || hover.Pane != 0 || hover.Index != 2 {
		t.Fatalf("left hover = %#v", msg)
	}
	m.Update(msg)
	if m.activePane != 0 || m.getTab().getPageSettings().cursor != 0 {
		t.Fatal("hover moved focus or cursor")
	}
	msg = m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseMove, X: 65, Y: 2})
	hover, ok = msg.(event.MouseHoverMsg)
	if !ok || hover.Pane != 1 || hover.Index != 0 || m.activePane != 0 {
		t.Fatalf("right hover = %#v, active pane = %d", msg, m.activePane)
	}
	msg = m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseMove, X: 49, Y: 2})
	hover, ok = msg.(event.MouseHoverMsg)
	if !ok || hover.Index != -1 {
		t.Fatalf("separator hover = %#v", msg)
	}
}

func TestSearchMouseHoverMapping(t *testing.T) {
	m := selectionModel(t)
	m.mode = searchMode
	m.search = newSearch(m)
	m.search.items = []searchItem{{path: "a"}, {path: "b"}}
	msg := m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseMove, X: 5, Y: 4})
	hover, ok := msg.(event.MouseHoverMsg)
	if !ok || !hover.Search || hover.Index != 1 {
		t.Fatalf("search hover = %#v", msg)
	}
	m.Update(msg)
	if m.hoverSearchIndex != 1 || m.search.cursor != 0 {
		t.Fatal("search hover moved cursor or mapped incorrectly")
	}
}

func TestPaneTabsHoverAndSwitchDirectly(t *testing.T) {
	m := selectionModel(t)
	m.panes[0].tabs = append(m.panes[0].tabs, newTab(`C:\other`, &page{}))
	secondX := paint.Width(paneTabLabels(m.panes[0])[0]) + 1
	msg := m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseMove, X: uint16(secondX), Y: 0})
	hover, ok := msg.(event.MouseHoverMsg)
	if !ok || !hover.Tab || hover.Pane != 0 || hover.Index != 1 {
		t.Fatalf("tab hover = %#v", msg)
	}
	msg = m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseDown, Button: term.MouseButtonLeft, X: uint16(secondX), Y: 0})
	click, ok := msg.(event.MouseTabMsg)
	if !ok || click.Pane != 0 || click.Index != 1 {
		t.Fatalf("tab click = %#v", msg)
	}
	m.Update(msg)
	if m.currentTab != 1 || m.mode != normalMode {
		t.Fatalf("direct tab switch produced tab=%d mode=%d", m.currentTab, m.mode)
	}
}

func TestOpenSelectionWithDefaultApp(t *testing.T) {
	m := selectionModel(t)
	var opened []string
	original := shellExecute
	t.Cleanup(func() { shellExecute = original })
	shellExecute = func(path, dir string) error {
		if dir != m.getTab().dir {
			t.Errorf("opened %s from %s", path, dir)
		}
		opened = append(opened, filepath.Base(path))
		if filepath.Base(path) == "d" {
			return errors.New("no association")
		}
		return nil
	}

	_, cmd := m.Update(event.KeyMsg{Name: "e"})
	applyEffect(m, cmd)
	if !slices.Equal(opened, []string{"a"}) {
		t.Fatalf("without marks opened %v, want the cursor item", opened)
	}

	opened = nil
	m.getPage().items[1].setSelected(true)
	m.getPage().items[3].setSelected(true)
	_, cmd = m.Update(event.KeyMsg{Name: "e"})
	applyEffect(m, cmd)
	if !slices.Equal(opened, []string{"b", "d"}) {
		t.Fatalf("opened %v, want the marked items", opened)
	}
	last := m.log[len(m.log)-1]
	if last.messageType != msgError || !strings.Contains(last.message, "d: no association") {
		t.Fatalf("failure not reported: %+v", last)
	}
	if m.mode != normalMode {
		t.Fatal("opening changed the mode")
	}
}
