package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/Fiend3d/catatui"
	"github.com/Fiend3d/catatui/term"
	"mc/internal/event"
	"mc/shutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testModel(t *testing.T, dirs ...string) *model {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir())
	m := initialModel(dirs)
	m.screenWidth = 100
	m.screenHeight = 24
	m.dimensions()
	t.Cleanup(func() {
		for _, task := range m.taskList {
			if task.cancel != nil {
				task.cancel()
			}
		}
		m.taskWorkers.Wait()
	})
	return &m
}
func keyEvent(m *model, name string) { m.Update(event.KeyMsg{Name: name}) }
func applyEffect(m *model, c event.Cmd) {
	if c == nil {
		return
	}
	switch v := c().(type) {
	case event.BatchMsg:
		for _, c := range v {
			applyEffect(m, c)
		}
	case event.SequenceMsg:
		for _, c := range v {
			applyEffect(m, c)
		}
	case event.Delay:
	case nil:
	default:
		_, next := m.Update(v)
		applyEffect(m, next)
	}
}
func finishTasks(t *testing.T, m *model) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for m.tasksPending() {
		select {
		case msg := <-m.taskEvents:
			_, cmd := m.Update(msg)
			applyEffect(m, cmd)
		case <-timer.C:
			t.Fatal("tasks did not finish")
		}
	}
}

func TestPaneInitializationSelectionAndIsolation(t *testing.T) {
	left, right, extra := t.TempDir(), t.TempDir(), t.TempDir()
	touch(t, filepath.Join(left, "a"))
	touch(t, filepath.Join(left, "b"))
	touch(t, filepath.Join(right, "r"))
	m := testModel(t, left, right, extra)
	if len(m.panes[0].tabs) != 2 || len(m.panes[1].tabs) != 1 {
		t.Fatal("incorrect startup tabs")
	}
	applyEffect(m, m.readTab(m.getTab()))
	keyEvent(m, "space")
	if !m.getTab().page.items[0].isSelected() {
		t.Fatal("space selection missing")
	}
	saved := m.getTab().getPageSettings().cursor
	keyEvent(m, "tab")
	if m.getTab().dir != right {
		t.Fatal("pane not switched")
	}
	applyEffect(m, m.readTab(m.getTab()))
	keyEvent(m, "insert")
	keyEvent(m, "tab")
	if m.getTab().getPageSettings().cursor != saved || !m.getTab().page.items[0].isSelected() {
		t.Fatal("pane state lost")
	}
	keyEvent(m, "shift+tab")
	if m.mode != jumpMode {
		t.Fatal("Jump key missing")
	}
	keyEvent(m, "esc")
	keyEvent(m, "v")
	if m.mode != normalMode {
		t.Fatal("Visual mode still reachable")
	}
}

func TestMouseWheelCanScrollCursorOffscreen(t *testing.T) {
	dir := t.TempDir()
	for i := range 12 {
		touch(t, filepath.Join(dir, fmt.Sprintf("%02d", i)))
	}
	m := testModel(t, dir)
	m.screenHeight = 10
	m.dimensions()
	applyEffect(m, m.readTab(m.getTab()))
	settings := m.getTab().getPageSettings()
	settings.cursor = 0
	m.handleWheel(3)
	if settings.start != 3 || settings.cursor != 0 {
		t.Fatalf("wheel produced start=%d cursor=%d", settings.start, settings.cursor)
	}
	backend := catatui.NewTestBackend(100, 10)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	if settings.start != 3 {
		t.Fatalf("render snapped viewport to cursor: start=%d", settings.start)
	}
	keyEvent(m, "down")
	if settings.start != 1 || settings.cursor != 1 {
		t.Fatalf("keyboard did not reveal cursor: start=%d cursor=%d", settings.start, settings.cursor)
	}
}

func TestCalculatedDirectorySizesRenderPersistAndSort(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "small")
	large := filepath.Join(dir, "large")
	unknown := filepath.Join(dir, "unknown")
	if err := os.MkdirAll(small, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(large, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(unknown, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(small, "data"), bytes.Repeat([]byte{'s'}, 128), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(large, "data"), bytes.Repeat([]byte{'l'}, 4096), 0644); err != nil {
		t.Fatal(err)
	}
	m := testModel(t, dir, dir)
	left := m.getTab()
	applyEffect(m, m.readTab(left))
	m.addJob()
	cmd := calculateSize(left, []string{small, large})
	keyEvent(m, "tab")
	applyEffect(m, cmd)
	if m.activePane != 1 {
		t.Fatal("calculation changed the active pane")
	}
	items := left.page.getItems()
	for _, it := range items {
		file := it.(*filepathItem)
		if file.isDir && file.name != "unknown" && file.sizeStr == "" {
			t.Fatalf("directory size missing for %s", file.name)
		}
	}
	applyEffect(m, m.readTab(left))
	for _, it := range left.page.getItems() {
		file := it.(*filepathItem)
		if file.isDir && file.name != "unknown" && file.sizeStr == "" {
			t.Fatalf("directory size lost on refresh for %s", file.name)
		}
	}
	left.sortItems(sizeSort, false)
	if left.page.getItems()[0].getName() != "unknown" || left.page.getItems()[1].getName() != "large" {
		t.Fatalf("descending size sort order starts %s, %s", left.page.getItems()[0].getName(), left.page.getItems()[1].getName())
	}
	left.sortItems(sizeSort, true)
	if left.page.getItems()[0].getName() != "unknown" || left.page.getItems()[1].getName() != "small" {
		t.Fatalf("ascending size sort order starts %s, %s", left.page.getItems()[0].getName(), left.page.getItems()[1].getName())
	}
	m.activePane, m.pane = 0, m.panes[0]
	backend := catatui.NewTestBackend(100, 24)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	if output := backend.Buffer().String(); !strings.Contains(output, "4.1 kB") || !strings.Contains(output, "128 B") {
		t.Fatalf("calculated sizes not rendered:\n%s", output)
	}
}
func TestStaleReadsAndSelectionsTargetOriginalTab(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "a"))
	m := testModel(t, dir)
	first := m.readTab(m.getTab())()
	latest := m.readTab(m.getTab())()
	m.Update(first)
	if m.getTab().page.items != nil {
		t.Fatal("stale refresh applied")
	}
	target := m.getTab()
	keyEvent(m, "tab")
	m.Update(latest)
	if len(target.page.items) != 1 || m.getTab().page.items != nil {
		t.Fatal("read populated wrong pane")
	}
	stale := m.readTab(target)()
	target.set(t.TempDir())
	m.Update(stale)
	if target.page.items != nil {
		t.Fatal("stale navigation applied")
	}
}
func TestBackgroundQueueUniqueNamesAndUndoRedo(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	file := filepath.Join(src, "a")
	touch(t, file)
	m := testModel(t, src, dst)
	for i := 0; i < 2; i++ {
		m.addJob()
		m.enqueue(newFileActionCommand(copyFileAction, []string{file}, dst, false), "execute")
	}
	if m.taskList[1].state != "queued" {
		t.Fatal("second task ran concurrently")
	}
	finishTasks(t, m)
	if !shutil.PathExists(filepath.Join(dst, "a")) || !shutil.PathExists(filepath.Join(dst, "a1")) {
		t.Fatal("queued destinations collided")
	}
	if m.jobs != 0 || len(m.cm.history) != 2 {
		t.Fatal("task accounting/history incorrect")
	}
	keyEvent(m, "u")
	finishTasks(t, m)
	if shutil.PathExists(filepath.Join(dst, "a1")) {
		t.Fatal("undo did not remove copied file")
	}
	keyEvent(m, "U")
	finishTasks(t, m)
	if !shutil.PathExists(filepath.Join(dst, "a1")) {
		t.Fatal("redo did not restore copied file")
	}
}
func TestDirectTransferCapturesSourceAndDestination(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	touch(t, filepath.Join(src, "a"))
	m := testModel(t, src, dst)
	applyEffect(m, m.readTab(m.getTab()))
	keyEvent(m, "Y")
	if m.mode != transferMode || m.input.Value() != dst {
		t.Fatal("destination prompt wrong")
	}
	keyEvent(m, "enter")
	keyEvent(m, "tab")
	finishTasks(t, m)
	if !shutil.PathExists(filepath.Join(dst, "a")) {
		t.Fatal("direct transfer failed")
	}
	if m.activePane != 1 {
		t.Fatal("completion stole focus")
	}
	if len(m.getTab().page.items) != 1 {
		t.Fatal("destination pane not refreshed")
	}
}
func TestPartialTaskUndoRedoOnlyCompletedFiles(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	touch(t, filepath.Join(src, "a"))
	if err := os.WriteFile(filepath.Join(src, "b"), bytes.Repeat([]byte("b"), 1024*1024), 0644); err != nil {
		t.Fatal(err)
	}
	c := newFileActionCommand(copyFileAction, []string{src}, dst, false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := c.executeTask(ctx, func(p shutil.Progress) {
		if p.Files == 1 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if err = c.undo(); err != nil {
		t.Fatal(err)
	}
	if err = c.redoTask(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dst, filepath.Base(src))
	if !shutil.PathExists(filepath.Join(output, "a")) || shutil.PathExists(filepath.Join(output, "b")) {
		t.Fatal("redo included unfinished work")
	}
}
func TestNativeViewsAndMouse(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	touch(t, filepath.Join(src, "界é.txt"))
	m := testModel(t, src, dst)
	m.bm = newBookmarks(nil)
	applyEffect(m, m.readTab(m.getTab()))
	for _, size := range [][2]uint16{{100, 24}, {40, 8}, {20, 5}} {
		backend := catatui.NewTestBackend(size[0], size[1])
		terminal, err := catatui.NewTerminal(backend)
		if err != nil {
			t.Fatal(err)
		}
		m.screenWidth, m.screenHeight = int(size[0]), int(size[1])
		if err = terminal.Draw(m.draw); err != nil {
			t.Fatal(err)
		}
		text := backend.Buffer().String()
		if size[0] < 40 && !strings.Contains(text, "Resize") {
			t.Fatal("missing small-terminal message")
		}
		if size[0] == 100 && !strings.Contains(text, "界é.txt") {
			t.Fatalf("Unicode filename missing: %s", text)
		}
	}
	m.screenWidth, m.screenHeight = 100, 24
	m.dimensions()
	msg := m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseDown, Button: term.MouseButtonLeft, X: 65, Y: 2})
	m.Update(msg)
	if m.activePane != 1 {
		t.Fatal("mouse did not focus right pane")
	}
	for _, mode := range []mode{helpMode, bookmarksMode, tabsMode, messagesMode, goMode, sortMode, copyMode, themeMode, transferMode} {
		m.mode = mode
		backend := catatui.NewTestBackend(80, 24)
		terminal, _ := catatui.NewTerminal(backend)
		m.screenWidth = 80
		if err := terminal.Draw(m.draw); err != nil {
			t.Fatalf("mode %d: %v", mode, err)
		}
	}
}
