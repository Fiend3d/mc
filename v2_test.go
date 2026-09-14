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
	"slices"
	"strconv"
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
	keyEvent(m, "ctrl+j")
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

func TestTaskStripReservesBottomRowOnlyWhileVisible(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "last-row-item"))
	m := testModel(t, dir)
	m.screenHeight = 10
	m.dimensions()
	applyEffect(m, m.readTab(m.getTab()))

	backend := catatui.NewTestBackend(100, 10)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(backend.Buffer().String(), "\n")
	if !strings.Contains(lines[len(lines)-1], "last-row-item") {
		t.Fatalf("unused bottom row without active task: %q", lines[len(lines)-1])
	}

	m.taskList = []*task{{id: 1, cmd: &fileActionCommand{}, state: "running"}}
	m.dimensions()
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	lines = strings.Split(backend.Buffer().String(), "\n")
	if !strings.Contains(lines[len(lines)-1], "running") {
		t.Fatalf("task strip missing from bottom row: %q", lines[len(lines)-1])
	}
}

func TestSizeScanSummaryAndStripPreference(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "an-item"))
	m := testModel(t, dir)
	m.screenHeight = 10
	m.dimensions()
	applyEffect(m, m.readTab(m.getTab()))

	scan := &task{
		id:       1,
		cmd:      newCalcSizeCommand(m.getTab(), []string{dir, dir}),
		state:    "scanning",
		readOnly: true,
		progress: shutil.Progress{Path: dir, Bytes: 4096, Files: 3, Steps: 1, TotalSteps: 4, Scanning: true},
	}
	// A walk cannot know its byte total ahead of time, so the counts are live
	// and the percentage comes from finished top-level entries.
	if got, want := taskSummary(scan), "#1 calculate size (2 dirs) · scanning · 4.1 kB in 3 files · 25%"; got != want {
		t.Fatalf("summary got %q, want %q", got, want)
	}

	backend := catatui.NewTestBackend(100, 10)
	terminal, _ := catatui.NewTerminal(backend)
	m.taskList = []*task{scan}
	m.dimensions()
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(backend.Buffer().String(), "\n")
	if !strings.Contains(lines[len(lines)-1], "calculate size") {
		t.Fatalf("lone scan missing from the strip: %q", lines[len(lines)-1])
	}

	// With file work active the strip belongs to the task that has a real
	// gauge, and the scan is only counted. The scan is first in the list to
	// prove the preference is not just list order.
	m.taskList = []*task{scan, {
		id:       2,
		cmd:      &fileActionCommand{},
		state:    "running",
		progress: shutil.Progress{Bytes: 50, Total: 100},
	}}
	m.dimensions()
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	strip := strings.Split(backend.Buffer().String(), "\n")
	last := strip[len(strip)-1]
	if strings.Contains(last, "calculate size") {
		t.Fatalf("scan took the strip from active file work: %q", last)
	}
	if !strings.Contains(last, "+1 more") {
		t.Fatalf("strip does not acknowledge the concurrent scan: %q", last)
	}
}

func TestTaskViewLaysOutColumnsAndTheSelectedDetail(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "an-item"))
	m := testModel(t, dir)
	m.taskList = []*task{
		{id: 1, cmd: &fileActionCommand{}, state: "completed",
			progress: shutil.Progress{Bytes: 100, Total: 100}},
		{id: 2, cmd: &deleteCommand{}, state: "failed", err: fmt.Errorf("access is denied")},
	}
	m.taskView, m.taskCursor = true, 1
	m.screenWidth, m.screenHeight = 90, 12
	m.dimensions()

	backend := catatui.NewTestBackend(90, 12)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(backend.Buffer().String(), "\n")

	if !strings.HasPrefix(lines[0], " 2 tasks") {
		t.Fatalf("header does not count the list: %q", lines[0])
	}
	// The state column is shared, so the states start at the same column no
	// matter how long the command names are.
	first, second := strings.Index(lines[1], "completed"), strings.Index(lines[2], "failed")
	if first < 0 || second < 0 || first != second {
		t.Fatalf("states are not in one column: %d vs %d (%q, %q)", first, second, lines[1], lines[2])
	}
	if !strings.HasPrefix(lines[2], " > [2]") {
		t.Fatalf("the cursor does not mark the selected task: %q", lines[2])
	}
	// The failure of the selected task is spelled out under the list, and the
	// keys that can act on it stay on the last row above the strip.
	if !strings.Contains(lines[len(lines)-2], "access is denied") {
		t.Fatalf("selected task's error is missing: %q", lines[len(lines)-2])
	}
	footer := lines[len(lines)-1]
	if !strings.Contains(footer, "Esc") || strings.Contains(footer, "cancel") {
		t.Fatalf("footer offers the wrong keys for a finished task: %q", footer)
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
	m.enqueueReadOnly(newCalcSizeCommand(left, []string{small, large}), "execute")
	keyEvent(m, "tab")
	finishTasks(t, m)
	if m.activePane != 1 {
		t.Fatal("calculation changed the active pane")
	}
	if m.jobs != 0 {
		t.Fatalf("job accounting leaked: %d", m.jobs)
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
	m := testModel(t, dir, t.TempDir()) // the second pane must hold a tab
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

// deepTree builds a directory with enough entries that a size walk is not
// instantaneous, so a cancellation has something to interrupt.
func deepTree(t *testing.T, root string) string {
	t.Helper()
	for i := 0; i < 40; i++ {
		dir := filepath.Join(root, "d"+strconv.Itoa(i))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for j := 0; j < 40; j++ {
			if err := os.WriteFile(filepath.Join(dir, "f"+strconv.Itoa(j)), []byte("x"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func TestTaskRatioFallsBackToStepsWithoutAByteTotal(t *testing.T) {
	cases := []struct {
		name     string
		progress shutil.Progress
		ratio    float64
		measured bool
	}{
		{"transfer uses bytes", shutil.Progress{Bytes: 50, Total: 100}, 0.5, true},
		{"transfer scan is unmeasured", shutil.Progress{Bytes: 50, Total: 100, Scanning: true}, 0, false},
		{"size walk uses steps", shutil.Progress{Steps: 3, TotalSteps: 4, Scanning: true}, 0.75, true},
		{"bytes win over steps", shutil.Progress{Bytes: 50, Total: 100, Steps: 1, TotalSteps: 4}, 0.5, true},
		{"empty directory is unmeasured", shutil.Progress{Scanning: true}, 0, false},
		{"ratio is clamped", shutil.Progress{Steps: 9, TotalSteps: 4, Scanning: true}, 1, true},
	}
	for _, tc := range cases {
		ratio, measured := taskRatio(tc.progress)
		if measured != tc.measured || ratio != tc.ratio {
			t.Fatalf("%s: got (%v, %v), want (%v, %v)", tc.name, ratio, measured, tc.ratio, tc.measured)
		}
	}
}

func TestSizeScanStepsDriveTheProgressBar(t *testing.T) {
	dir := t.TempDir()
	roots := []string{filepath.Join(dir, "one"), filepath.Join(dir, "two")}
	for _, root := range roots {
		// Three top-level entries each: two subdirectories and a loose file.
		for _, sub := range []string{"a", "b"} {
			if err := os.MkdirAll(filepath.Join(root, sub), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, sub, "data"), []byte("xy"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(root, "loose"), []byte("z"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	m := testModel(t, dir)
	c := newCalcSizeCommand(m.getTab(), roots)
	var reports []shutil.Progress
	if err := c.executeTask(context.Background(), func(p shutil.Progress) {
		reports = append(reports, p)
	}); err != nil {
		t.Fatal(err)
	}

	// Six top-level entries across both roots, counted by one readdir each.
	for i, p := range reports {
		if p.TotalSteps != 6 {
			t.Fatalf("report %d has TotalSteps %d, want 6", i, p.TotalSteps)
		}
		if p.Total != 0 {
			t.Fatalf("report %d claims a byte total: %+v", i, p)
		}
		if i > 0 && p.Steps < reports[i-1].Steps {
			t.Fatalf("steps went backwards at %d: %d after %d", i, p.Steps, reports[i-1].Steps)
		}
	}
	last := reports[len(reports)-1]
	if last.Steps != 6 {
		t.Fatalf("final report reached %d of 6 steps", last.Steps)
	}
	if ratio, measured := taskRatio(last); !measured || ratio != 1 {
		t.Fatalf("finished scan is not a full bar: (%v, %v)", ratio, measured)
	}
	// The bar must actually move partway through, not jump 0 -> 100.
	middle := false
	for _, p := range reports {
		if r, _ := taskRatio(p); r > 0 && r < 1 {
			middle = true
		}
	}
	if !middle {
		t.Fatal("the bar never showed intermediate progress")
	}
}

func TestSizeScanStripRendersAGauge(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "an-item"))
	m := testModel(t, dir)
	m.screenHeight = 10
	m.dimensions()
	applyEffect(m, m.readTab(m.getTab()))

	m.taskList = []*task{{
		id:       1,
		cmd:      newCalcSizeCommand(m.getTab(), []string{dir}),
		state:    "scanning",
		readOnly: true,
		progress: shutil.Progress{Path: dir, Bytes: 4096, Files: 3, Steps: 3, TotalSteps: 4, Scanning: true},
	}}
	m.dimensions()
	backend := catatui.NewTestBackend(100, 10)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(backend.Buffer().String(), "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, "75%") {
		t.Fatalf("step percentage missing from the strip: %q", last)
	}
	if !strings.Contains(last, "calculate size") {
		t.Fatalf("scan summary missing from the strip: %q", last)
	}
}

func TestGoModeSKeySubmitsAConcurrentSizeTask(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "measured")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "data"), bytes.Repeat([]byte{'x'}, 512), 0644); err != nil {
		t.Fatal(err)
	}
	m := testModel(t, dir)
	applyEffect(m, m.readTab(m.getTab()))
	for _, it := range m.getPage().getItems() {
		if it.getName() == "measured" {
			it.setSelected(true)
		}
	}

	keyEvent(m, "g")
	keyEvent(m, "s")
	if len(m.taskList) != 1 {
		t.Fatalf("g+s did not submit a task, taskList has %d", len(m.taskList))
	}
	scan := m.taskList[0]
	if !scan.readOnly {
		t.Fatal("the size task is not marked read-only, so it would occupy the file queue")
	}
	if scan.state == "queued" {
		t.Fatalf("the size task was queued, state %q", scan.state)
	}
	if m.jobs != 1 {
		t.Fatalf("expected exactly one job registered, got %d", m.jobs)
	}

	finishTasks(t, m)
	if m.jobs != 0 {
		t.Fatalf("job accounting leaked: %d", m.jobs)
	}
	for _, it := range m.getPage().getItems() {
		if it.getName() != "measured" {
			continue
		}
		if file := it.(*filepathItem); file.size != 512 {
			t.Fatalf("size not applied to the pane: %d (%q)", file.size, file.sizeStr)
		}
	}
}

func TestSizeScanProgressAccumulatesAcrossDirectories(t *testing.T) {
	dir := t.TempDir()
	small, large := filepath.Join(dir, "small"), filepath.Join(dir, "large")
	for _, d := range []string{small, large} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(small, "data"), bytes.Repeat([]byte{'s'}, 128), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(large, "data"), bytes.Repeat([]byte{'l'}, 4096), 0644); err != nil {
		t.Fatal(err)
	}

	m := testModel(t, dir)
	c := newCalcSizeCommand(m.getTab(), []string{small, large})
	var reports []shutil.Progress
	if err := c.executeTask(context.Background(), func(p shutil.Progress) {
		reports = append(reports, p)
	}); err != nil {
		t.Fatal(err)
	}

	if c.total != 128+4096 {
		t.Fatalf("total %d", c.total)
	}
	if len(c.results) != 2 {
		t.Fatalf("expected both directories measured, got %d", len(c.results))
	}
	// The counters must carry across directory boundaries rather than restart,
	// otherwise the strip visibly counts backwards partway through.
	for i, p := range reports {
		if i > 0 && (p.Bytes < reports[i-1].Bytes || p.Files < reports[i-1].Files) {
			t.Fatalf("progress reset at report %d: %+v after %+v", i, p, reports[i-1])
		}
	}
	last := reports[len(reports)-1]
	if last.Bytes != 128+4096 || last.Files != 2 {
		t.Fatalf("final report %+v does not cover both directories", last)
	}
}

func TestSizeScanRunsBesideFileTasks(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	file := filepath.Join(src, "a")
	touch(t, file)
	tree := deepTree(t, t.TempDir())

	// The scan is submitted first: the copy must not be stuck behind it.
	m := testModel(t, src, dst)
	m.addJob()
	m.enqueueReadOnly(newCalcSizeCommand(m.getTab(), []string{tree}), "execute")
	m.addJob()
	m.enqueue(newFileActionCommand(copyFileAction, []string{file}, dst, false), "execute")
	if m.taskList[0].state == "queued" {
		t.Fatal("read-only scan was queued")
	}
	if m.taskList[1].state == "queued" {
		t.Fatal("file task queued behind a read-only scan")
	}
	if !m.mutatingPending() {
		t.Fatal("the copy should count as pending file work")
	}
	finishTasks(t, m)
	if m.jobs != 0 {
		t.Fatalf("job accounting leaked: %d", m.jobs)
	}
	if !shutil.PathExists(filepath.Join(dst, "a")) {
		t.Fatal("the copy did not run")
	}

	// And the mirror ordering: a running copy must not delay a scan.
	m2 := testModel(t, src, dst)
	m2.addJob()
	m2.enqueue(newFileActionCommand(copyFileAction, []string{file}, dst, false), "execute")
	m2.addJob()
	m2.enqueueReadOnly(newCalcSizeCommand(m2.getTab(), []string{tree}), "execute")
	if m2.taskList[1].state == "queued" {
		t.Fatal("scan queued behind a running file task")
	}
	finishTasks(t, m2)
	if m2.jobs != 0 {
		t.Fatalf("job accounting leaked: %d", m2.jobs)
	}
}

func TestSizeScanCancelDoesNotBlockUndo(t *testing.T) {
	dir := t.TempDir()
	tree := deepTree(t, filepath.Join(dir, "tree"))
	m := testModel(t, dir, t.TempDir())
	applyEffect(m, m.readTab(m.getTab()))
	m.addJob()
	m.enqueueReadOnly(newCalcSizeCommand(m.getTab(), []string{tree}), "execute")

	// A read-only scan must not stand in the way of undo/redo.
	if m.mutatingPending() {
		t.Fatal("a size scan counts as pending file work")
	}

	m.taskView, m.taskCursor = true, 0
	keyEvent(m, "c")
	if m.taskList[0].state != "cancelling" {
		t.Fatalf("c did not cancel the scan, state %q", m.taskList[0].state)
	}
	finishTasks(t, m)
	// The worker may have finished before the cancel landed, so either terminal
	// state is legitimate; what matters is that the bookkeeping is clean.
	if s := m.taskList[0].state; s != "cancelled" && s != "completed" {
		t.Fatalf("unexpected terminal state %q", s)
	}
	if m.tasksPending() || m.jobs != 0 {
		t.Fatalf("cancelled scan left bookkeeping behind: pending=%v jobs=%d", m.tasksPending(), m.jobs)
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
func TestCreatedItemIsSelected(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		touch(t, filepath.Join(dir, name))
	}
	m := testModel(t, dir, t.TempDir())
	applyEffect(m, m.readTab(m.getTab()))
	m.sort(alphabeticSort, false)
	for _, name := range []string{"zz.txt", "new dir\\"} {
		keyEvent(m, "a")
		m.input.SetValue(name)
		keyEvent(m, "enter")
		finishTasks(t, m)
		want := filepath.Join(dir, strings.TrimSuffix(name, "\\"))
		tab := m.getTab()
		if got := tab.page.getItems()[tab.getPageSettings().cursor].getFullPath(); got != want {
			t.Fatalf("after creating %q the cursor is on %q", name, got)
		}
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

func TestTabMovesAndCopiesBetweenPanes(t *testing.T) {
	left, right, extra := t.TempDir(), t.TempDir(), t.TempDir()
	touch(t, filepath.Join(left, "a"))
	m := testModel(t, left, right, extra)

	keyEvent(m, "ctrl+left")
	if len(m.panes[0].tabs) != 2 || len(m.panes[1].tabs) != 1 || m.activePane != 0 {
		t.Fatal("moving towards the focused pane changed something")
	}

	keyEvent(m, "ctrl+right")
	if len(m.panes[0].tabs) != 1 || m.panes[0].tabs[0].dir != extra {
		t.Fatal("tab not taken from the source pane")
	}
	if len(m.panes[1].tabs) != 2 || m.panes[1].currentTab != 1 {
		t.Fatal("moved tab is not current in the destination pane")
	}
	if m.activePane != 1 || m.pane != m.panes[1] || m.getTab().dir != left {
		t.Fatal("focus did not follow the moved tab")
	}

	keyEvent(m, "ctrl+left")
	if len(m.panes[0].tabs) != 2 || m.activePane != 0 || m.getTab().dir != left {
		t.Fatal("tab did not move back")
	}

	keyEvent(m, "tab")
	keyEvent(m, "ctrl+left")
	if len(m.panes[1].tabs) != 0 || m.panes[1].currentTab != 0 {
		t.Fatal("a pane kept its last tab")
	}
	if len(m.panes[0].tabs) != 3 || m.activePane != 0 {
		t.Fatal("focus did not follow the last tab out of the pane")
	}
	keyEvent(m, "shift+right") // refill the emptied pane
	keyEvent(m, "ctrl+w")      // and drop the duplicate on this side
	if len(m.panes[0].tabs) != 2 || len(m.panes[1].tabs) != 1 || m.getTab().dir != left {
		t.Fatal("could not restore two populated panes")
	}

	_, cmd := m.Update(event.KeyMsg{Name: "shift+right"})
	applyEffect(m, cmd)
	if len(m.panes[0].tabs) != 2 || m.activePane != 0 || m.getTab().dir != left {
		t.Fatal("copying moved the tab or the focus")
	}
	target := m.panes[1]
	if len(target.tabs) != 2 || target.currentTab != 1 || target.tabs[1].dir != left {
		t.Fatal("tab not copied to the other pane")
	}
	if len(target.tabs[1].page.items) != 1 {
		t.Fatal("copied tab was not read")
	}
	if m.getTab() == target.tabs[1] {
		t.Fatal("copy shares the tab with the source pane")
	}
}

// drawText renders the model and returns the screen as text.
func drawText(t *testing.T, m *model) string {
	t.Helper()
	backend := catatui.NewTestBackend(100, 24)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	return backend.Buffer().String()
}

func TestPaneCanBeLeftWithoutTabs(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	touch(t, filepath.Join(left, "a"))
	m := testModel(t, left, right)
	applyEffect(m, m.readTab(m.getTab()))

	keyEvent(m, "ctrl+w")
	if m.hasTabs() || len(m.panes[0].tabs) != 0 || m.currentTab != 0 {
		t.Fatal("closing the last tab did not empty the pane")
	}

	// Every key that needs a current tab must be swallowed, not panic.
	for _, key := range []string{
		"j", "k", "l", "h", "down", "up", "left", "right", "enter", "space", "insert",
		"home", "end", "pgup", "pgdown", "shift+up", "shift+down", "ctrl+a", "ctrl+d",
		"ctrl+r", "d", "e", "r", "y", "x", "p", "P", "f", ",", "a", "c", "s", ":", "B",
		"[", "]", "1", "0", "ctrl+n", "ctrl+t", "ctrl+b", "ctrl+f", "ctrl+w",
		"f2", "f3", "f5", "esc",
	} {
		keyEvent(m, key)
		if m.hasTabs() {
			t.Fatalf("%q created a tab in an empty pane", key)
		}
		if m.mode != normalMode {
			t.Fatalf("%q changed the mode in an empty pane", key)
		}
	}
	m.Update(event.MouseClickMsg{X: 4, Y: 4, Button: event.MouseLeft})
	m.Update(event.MouseWheelMsg{X: 4, Y: 4, Button: event.MouseWheelDown})

	if out := drawText(t, m); !strings.Contains(out, "no tabs") {
		t.Fatal("empty pane is not drawn")
	}
	if out := drawText(t, m); !strings.Contains(out, filepath.Base(right)) {
		t.Fatal("the populated pane stopped rendering")
	}

	keyEvent(m, "q")
	if m.result != "" {
		t.Fatalf("quitting an empty pane returned %q", m.result)
	}
}

func TestEmptyPaneCanBeRefilled(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	m := testModel(t, left, right)

	keyEvent(m, "ctrl+w")
	_, cmd := m.Update(event.KeyMsg{Name: "T"})
	applyEffect(m, cmd)
	if !m.hasTabs() || m.getTab().dir != left {
		t.Fatal("T did not restore a tab into the empty pane")
	}

	keyEvent(m, "ctrl+w")
	keyEvent(m, "tab")
	_, cmd = m.Update(event.KeyMsg{Name: "shift+left"})
	applyEffect(m, cmd)
	if len(m.panes[0].tabs) != 1 || m.panes[0].tabs[0].dir != right {
		t.Fatal("shift+left did not copy a tab into the empty pane")
	}
	if m.activePane != 1 {
		t.Fatal("copying moved the focus")
	}

	keyEvent(m, "tab")
	keyEvent(m, "ctrl+w")
	keyEvent(m, "g")
	keyEvent(m, "g")
	if m.mode != pathMode {
		t.Fatal("gg is unreachable from an empty pane")
	}
	m.pathInput.SetValue(left)
	_, cmd = m.Update(event.KeyMsg{Name: "enter"})
	applyEffect(m, cmd)
	if !m.hasTabs() || m.getTab().dir != left {
		t.Fatal("path mode did not open a tab in the empty pane")
	}
}

func TestBothPanesCanBeEmpty(t *testing.T) {
	m := testModel(t, t.TempDir(), t.TempDir())
	keyEvent(m, "ctrl+w")
	keyEvent(m, "tab")
	keyEvent(m, "ctrl+w")
	if m.panes[0].hasTabs() || m.panes[1].hasTabs() {
		t.Fatal("panes are not both empty")
	}
	if out := drawText(t, m); strings.Count(out, "no tabs") < 2 {
		t.Fatal("both empty panes are not drawn")
	}
	keyEvent(m, "tab")
	if m.activePane != 0 {
		t.Fatal("Tab does not move between empty panes")
	}
	keyEvent(m, "g")
	keyEvent(m, "t")
	if m.mode != tabsMode {
		t.Fatal("the tab browser is unreachable")
	}
	if out := drawText(t, m); !strings.Contains(out, "no tabs in this pane") {
		t.Fatal("the tab browser does not report an empty pane")
	}
	keyEvent(m, "esc")
}

func TestTransferPrefillWithEmptyOppositePane(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	touch(t, filepath.Join(left, "a"))
	m := testModel(t, left, right)
	applyEffect(m, m.readTab(m.getTab()))

	keyEvent(m, "tab")
	keyEvent(m, "ctrl+w")
	keyEvent(m, "tab")
	keyEvent(m, "Y")
	if m.mode != transferMode {
		t.Fatal("Y did not open the transfer prompt")
	}
	if m.input.Value() != "" {
		t.Fatalf("destination prefilled with %q", m.input.Value())
	}
	keyEvent(m, "esc")
}

func TestSingleDirectoryStartsOnlyTheLeftPane(t *testing.T) {
	left, right, extra := t.TempDir(), t.TempDir(), t.TempDir()

	m := testModel(t, left)
	if len(m.panes[0].tabs) != 1 || m.panes[0].tabs[0].dir != left {
		t.Fatal("left pane not seeded")
	}
	if m.panes[1].hasTabs() {
		t.Fatal("right pane opened the same directory twice")
	}
	if m.activePane != 0 || !m.hasTabs() {
		t.Fatal("focus should start on the populated left pane")
	}
	keyEvent(m, "q")
	if m.result != left {
		t.Fatalf("quit returned %q", m.result)
	}

	m = testModel(t, left, right)
	if len(m.panes[0].tabs) != 1 || len(m.panes[1].tabs) != 1 || m.panes[1].tabs[0].dir != right {
		t.Fatal("two directories no longer initialize both panes")
	}

	m = testModel(t, left, right, extra)
	if len(m.panes[0].tabs) != 2 || m.panes[0].tabs[1].dir != extra {
		t.Fatal("extra directories are no longer left-pane tabs")
	}
}

// typeKeys sends printable characters through the model one at a time.
func typeKeys(m *model, text string) {
	for _, r := range text {
		m.Update(event.KeyMsg{Name: string(r), Text: string(r)})
	}
}

func TestPathModeCompletesDrivesWithoutACurrentDirectory(t *testing.T) {
	drives, err := getDrives()
	if err != nil || len(drives) == 0 {
		t.Skip("no drives reported")
	}
	first := newDriveItem(drives[0]).getFullPath()

	m := testModel(t, "") // a This PC tab: dir is empty
	if m.getTab().dir != "" {
		t.Fatal("expected a This PC tab")
	}
	keyEvent(m, "g")
	keyEvent(m, "g")
	if m.mode != pathMode {
		t.Fatal("gg did not open path mode")
	}
	typeKeys(m, first[:1])
	if !m.pathInput.ShowSuggestions {
		t.Fatal("suggestions are off on a This PC tab")
	}
	if got := m.pathInput.MatchedSuggestions(); len(got) == 0 || got[0] != first {
		t.Fatalf("drive not suggested, got %v", got)
	}
	if m.pathInput.CurrentSuggestion() != first {
		t.Fatalf("tab would not complete to %q", first)
	}

	// Once a separator is typed the normal directory listing takes over.
	typeKeys(m, first[1:])
	if m.pathInputDir != first {
		t.Fatalf("still completing drives at %q", m.pathInputDir)
	}
}

func TestPathModeIsVisibleInAnEmptyPane(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	m := testModel(t, left, right)
	keyEvent(m, "ctrl+w")
	if m.hasTabs() {
		t.Fatal("pane not emptied")
	}
	keyEvent(m, "g")
	keyEvent(m, "g")
	typeKeys(m, "Z:")
	out := drawText(t, m)
	if !strings.Contains(out, "Z:") {
		t.Fatal("the path input is not drawn in an empty pane")
	}
	if !strings.Contains(out, "PATH") {
		t.Fatal("the empty pane does not show the mode")
	}
	keyEvent(m, "esc")
}

func TestPathModeStillCompletesInsideADirectory(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(wd) }) // path mode chdirs; release the temp dir
	if err := os.MkdirAll(filepath.Join(dir, "childdir"), 0755); err != nil {
		t.Fatal(err)
	}
	m := testModel(t, dir, t.TempDir())
	keyEvent(m, "g")
	keyEvent(m, "g")
	if m.pathInput.Value() != dir {
		t.Fatalf("path mode opened with %q", m.pathInput.Value())
	}
	typeKeys(m, `\child`) // the input is prefilled with the current directory
	want := filepath.Join(dir, "childdir")
	if got := m.pathInput.CurrentSuggestion(); got != want {
		t.Fatalf("suggestion = %q, want %q", got, want)
	}
}

func TestTabsListRoundTrip(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if got, err := loadTabs(); err != nil || got != nil {
		t.Fatalf("missing file returned %v, %v", got, err)
	}
	want := []string{`C:\one`, "", `C:\two`} // "" is the This PC view
	if err := saveTabs(want); err != nil {
		t.Fatal(err)
	}
	got, err := loadTabs()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("loaded %q, want %q", got, want)
	}
	if err := saveTabs(nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := loadTabs(); len(got) != 0 {
		t.Fatalf("an emptied pane loaded %q", got)
	}
}

func TestRightPaneRestoresSavedTabs(t *testing.T) {
	appData, kept, gone := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	// initialModel reads APPDATA, so seed it here rather than via testModel.
	t.Setenv("APPDATA", appData)
	if err := saveTabs([]string{kept, gone, ""}); err != nil {
		t.Fatal(err)
	}

	left := t.TempDir()
	m := initialModel([]string{left})
	if len(m.panes[0].tabs) != 1 || m.panes[0].tabs[0].dir != left {
		t.Fatal("the left pane should ignore saved tabs")
	}
	if len(m.panes[1].tabs) != 2 {
		t.Fatalf("right pane restored %d tabs", len(m.panes[1].tabs))
	}
	if m.panes[1].tabs[0].dir != kept || m.panes[1].tabs[1].dir != "" {
		t.Fatal("a missing directory was restored, or This PC was dropped")
	}

	// An explicit second argument wins over the saved list.
	right := t.TempDir()
	m = initialModel([]string{left, right})
	if len(m.panes[1].tabs) != 1 || m.panes[1].tabs[0].dir != right {
		t.Fatal("the argument did not win")
	}
}

func TestSaveTabsWritesTheRightPane(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	m := testModel(t, left, right) // sets APPDATA to a temp dir of its own

	keyEvent(m, "ctrl+right") // send the left tab across
	m.saveTabs()
	got, err := loadTabs()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{right, left}) {
		t.Fatalf("saved %q, want %q", got, []string{right, left})
	}

	keyEvent(m, "ctrl+w")
	keyEvent(m, "ctrl+w")
	if m.panes[1].hasTabs() {
		t.Fatal("right pane not emptied")
	}
	m.saveTabs()
	if got, _ := loadTabs(); len(got) != 0 {
		t.Fatalf("emptied pane saved %q", got)
	}
}

func TestOnlyTheRightPaneAdvertisesSaving(t *testing.T) {
	m := testModel(t, t.TempDir(), t.TempDir())
	keyEvent(m, "tab")
	keyEvent(m, "ctrl+w")
	if !strings.Contains(drawText(t, m), "saved for the next launch") {
		t.Fatal("the empty right pane does not mention saving")
	}

	m = testModel(t, t.TempDir(), t.TempDir())
	keyEvent(m, "ctrl+w")
	if strings.Contains(drawText(t, m), "saved for the next launch") {
		t.Fatal("the empty left pane claims persistence it does not have")
	}
}
