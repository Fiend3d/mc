package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Fiend3d/catatui"
	"github.com/Fiend3d/catatui/term"
	"github.com/fsnotify/fsnotify"
	"mc/internal/event"
)

func TestRefreshPreservesFileIdentityAndFilteredViewport(t *testing.T) {
	m := selectionModel(t)
	tab := m.getTab()
	tab.filterText = []string{""}
	tab.filter()
	s := tab.getPageSettings()
	s.cursor, s.start = 2, 1 // c, with b at the top
	path := tab.page.getItems()[s.cursor].getFullPath()
	touch(t, filepath.Join(tab.dir, "aa"))
	applyEffect(m, m.readTab(tab))
	if got := tab.page.getItems()[s.cursor].getFullPath(); got != path {
		t.Fatalf("refresh moved cursor to %s, want %s", got, path)
	}
	if got := tab.page.getItems()[s.start].getName(); got != "b" {
		t.Fatalf("refresh moved viewport to %s, want b", got)
	}
}

func TestRefreshQueueRetainsTrailingChangeAndWaitsForSlowRead(t *testing.T) {
	m := selectionModel(t)
	tab := m.getTab()
	dir := tab.dir
	q := make(refreshQueue)
	watch := map[string]bool{dir: true}
	now := time.Now()
	touch(t, filepath.Join(dir, "first"))
	q.changed(fsnotify.Event{Name: filepath.Join(dir, "first"), Op: fsnotify.Create}, watch, now)
	if cmds := q.drain(m, now); len(cmds) != 0 {
		t.Fatal("batch read too early")
	}
	cmds := q.drain(m, now.Add(100*time.Millisecond))
	if len(cmds) != 2 {
		t.Fatalf("got %d reads for two tabs", len(cmds))
	}
	// Capture stale snapshots, then change the directory while their results
	// are still in flight. The next refresh must wait, not invalidate them.
	var results []event.Msg
	for _, cmd := range cmds {
		results = append(results, cmd())
	}
	touch(t, filepath.Join(dir, "last"))
	q.changed(fsnotify.Event{Name: filepath.Join(dir, "last"), Op: fsnotify.Create}, watch, now.Add(110*time.Millisecond))
	if more := q.drain(m, now.Add(time.Second)); len(more) != 0 || len(q) != 1 {
		t.Fatal("busy read lost its pending follow-up")
	}
	for _, msg := range results {
		m.Update(msg)
	}
	for _, cmd := range q.drain(m, now.Add(2*time.Second)) {
		applyEffect(m, cmd)
	}
	found := false
	for _, it := range tab.page.items {
		if it.getName() == "last" {
			found = true
		}
	}
	if !found || len(q) != 0 || tab.pendingReads != 0 {
		t.Fatal("trailing change was lost")
	}
}

func TestUnchangedAutomaticRefreshKeepsRangeAndSort(t *testing.T) {
	m := selectionModel(t)
	tab := m.getTab()
	tab.sortItems(alphabeticSort, true)
	keyEvent(m, "home")
	keyEvent(m, "shift+down")
	r := tab.page.selectionRange
	applyEffect(m, m.readTabMode(tab, true))
	if tab.page.selectionRange != r {
		t.Fatal("unchanged polling ended the range")
	}
	assertSelection(t, m, "ed")
	keyEvent(m, "shift+up")
	assertSelection(t, m, "e")
}

func TestDirectoryReadErrorsAreTargetedAndDoNotBlockRetry(t *testing.T) {
	m := selectionModel(t)
	tab := m.getTab()
	tab.set(filepath.Join(tab.dir, "not-created-yet"))
	cmd := m.readTabMode(tab, true)
	applyEffect(m, cmd)
	if tab.pendingReads != 0 || tab.lastReadError == "" {
		t.Fatal("read failure did not finish")
	}
	logs := len(m.log)
	applyEffect(m, m.readTabMode(tab, true))
	if len(m.log) != logs {
		t.Fatal("automatic retry spammed the same error")
	}
	if err := os.Mkdir(tab.dir, 0755); err != nil {
		t.Fatal(err)
	}
	touch(t, filepath.Join(tab.dir, "recovered"))
	applyEffect(m, m.readTabMode(tab, true))
	if tab.lastReadError != "" || len(tab.page.items) != 1 {
		t.Fatal("read did not recover")
	}
	tab.set(filepath.Join(tab.dir, "missing"))
	stale := m.readTab(tab)()
	tab.set(t.TempDir())
	logs = len(m.log)
	m.Update(stale)
	if len(m.log) != logs {
		t.Fatal("stale directory error reached the new tab location")
	}
}

func TestOverlayMouseCannotChangeUnderlyingPane(t *testing.T) {
	for _, overlay := range []string{"tasks", "quit"} {
		t.Run(overlay, func(t *testing.T) {
			m := selectionModel(t)
			m.taskView = overlay == "tasks"
			m.quitting = overlay == "quit"
			msg := m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseDown, Button: term.MouseButtonLeft, X: 52, Y: 0})
			if msg != nil || m.activePane != 0 {
				t.Fatalf("overlay click reached pane: message=%#v pane=%d", msg, m.activePane)
			}
		})
	}
}

func TestRightClickDoesNotActivateTab(t *testing.T) {
	m := selectionModel(t)
	msg := m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseDown, Button: term.MouseButtonRight, X: 52, Y: 0})
	if msg != nil || m.activePane != 0 {
		t.Fatalf("right click activated tab: %#v pane=%d", msg, m.activePane)
	}
}

func TestBreadcrumbWideCharactersNavigateToHoveredComponent(t *testing.T) {
	m := testModel(t, `C:\界\two\leaf`)
	// "two" begins at column 6; its final character is at column 8.
	_, cmd := m.Update(event.MouseClickMsg{X: 8, Y: 0, Button: event.MouseLeft})
	if cmd == nil || m.getTab().dir != `C:\界\two` {
		t.Fatalf("breadcrumb navigated to %q", m.getTab().dir)
	}
}

func TestFilesystemNotificationsRefreshBothPanes(t *testing.T) {
	m := selectionModel(t)
	applyEffect(m, m.readTab(m.panes[1].tabs[0]))
	dir := m.getTab().dir
	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Add(dir); err != nil {
		t.Fatal(err)
	}
	q := make(refreshQueue)
	watch := map[string]bool{dir: true}
	path := filepath.Join(dir, "external-file")
	for _, create := range []bool{true, false} {
		if create {
			touch(t, path)
		} else if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		timeout := time.NewTimer(5 * time.Second)
		tick := time.NewTicker(20 * time.Millisecond)
		complete := false
		for !complete {
			select {
			case e := <-w.Events:
				q.changed(e, watch, time.Now())
			case err := <-w.Errors:
				t.Fatal(err)
			case <-timeout.C:
				t.Fatal("filesystem notification did not refresh both panes")
			case now := <-tick.C:
				for _, cmd := range q.drain(m, now) {
					applyEffect(m, cmd)
				}
				complete = true
				for _, p := range m.panes {
					found := false
					for _, it := range p.tabs[0].page.items {
						if it.getFullPath() == path {
							found = true
						}
					}
					if found != create {
						complete = false
					}
				}
			}
		}
		timeout.Stop()
		tick.Stop()
	}
}

func TestExplicitCursorPlacementVisibleWithoutBreakingWheelScroll(t *testing.T) {
	m := selectionModel(t)
	m.screenHeight = 8
	m.dimensions()
	tab := m.getTab()
	_, _ = m.Update(selectItemMsg{target: tab, page: tab.page, path: filepath.Join(tab.dir, "e")})
	s := tab.getPageSettings()
	if s.cursor != 4 || s.start != 2 {
		t.Fatalf("placed cursor=%d start=%d", s.cursor, s.start)
	}
	m.hoverPane, m.hoverIndex = 0, 4
	m.Update(event.BlurMsg{})
	if m.hoverIndex != -1 {
		t.Fatal("blur left a stale hover")
	}
	// A shortened listing must not leave nearly the whole viewport empty.
	tab.page.items = tab.page.items[:3]
	b := catatui.NewTestBackend(100, 8)
	terminal, _ := catatui.NewTerminal(b)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	if s.start != 0 {
		t.Fatal("viewport was not clamped after listing shrank")
	}
}
