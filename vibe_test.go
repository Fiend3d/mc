package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Fiend3d/catatui"
	"github.com/Fiend3d/catatui/term"
	"mc/internal/event"
)

func vibeTestGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func vibeWrite(t *testing.T, root, path, text string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func vibeRepo(t *testing.T) string {
	t.Helper()
	if gitBinary() == "" {
		t.Skip("git missing")
	}
	root := t.TempDir()
	vibeTestGit(t, root, "init", "-b", "main")
	vibeTestGit(t, root, "config", "core.autocrlf", "false")
	return root
}
func vibeCommit(t *testing.T, root string) {
	t.Helper()
	vibeTestGit(t, root, "add", ".")
	vibeTestGit(t, root, "-c", "commit.gpgsign=false", "commit", "-m", "fixture")
}
func vibeRead(t *testing.T, root string) vibeSnapshot {
	t.Helper()
	s, err := readVibe(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func vibeFind(t *testing.T, s vibeSnapshot, path string) vibeFile {
	t.Helper()
	for _, f := range s.files {
		if f.path == path {
			return f
		}
	}
	t.Fatalf("missing file %q: %#v", path, s.files)
	return vibeFile{}
}

func TestVibeRepositoryDiffAndRootTree(t *testing.T) {
	root := vibeRepo(t)
	vibeWrite(t, root, "src/deep/main.go", "alpha\nold\nomega\n")
	vibeWrite(t, root, "gone.txt", "deleted one\ndeleted two\n")
	vibeWrite(t, root, "rename.txt", strings.Repeat("unchanged\n", 10))
	vibeWrite(t, root, "cancelled.txt", "original\n")
	vibeWrite(t, root, ".gitignore", "ignored/\n")
	vibeCommit(t, root)
	vibeWrite(t, root, "src/deep/main.go", "alpha\nstaged\nomega\n")
	vibeTestGit(t, root, "add", "src/deep/main.go")
	vibeWrite(t, root, "src/deep/main.go", "alpha\nworking\nomega\n")
	vibeWrite(t, root, "cancelled.txt", "staged\n")
	vibeTestGit(t, root, "add", "cancelled.txt")
	vibeWrite(t, root, "cancelled.txt", "original\n")
	if err := os.Remove(filepath.Join(root, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	vibeTestGit(t, root, "mv", "rename.txt", "renamed.txt")
	vibeWrite(t, root, "new/deep/你好 file.txt", "new line\n")
	vibeWrite(t, root, "ignored/no.txt", "ignored")
	s := vibeRead(t, root)
	if len(s.files) != 4 {
		t.Fatalf("files = %#v", s.files)
	}
	f := vibeFind(t, s, "src/deep/main.go")
	if f.added != 1 || f.deleted != 1 || len(f.hunks) != 1 {
		t.Fatalf("diff = %#v", f)
	}
	lines := f.hunks[0].lines
	if len(lines) != 4 || lines[1].text != "old" || lines[1].old != 2 || lines[2].text != "working" || lines[2].new != 2 {
		t.Fatalf("line mappings %v", lines)
	}
	rename := vibeFind(t, s, "renamed.txt")
	if rename.oldPath != "rename.txt" || rename.status != "R" {
		t.Fatalf("rename %v", rename)
	}
	deleted := vibeFind(t, s, "gone.txt")
	if deleted.deleted != 2 || deleted.blob == "" {
		t.Fatal("missing historical deletion")
	}
	m := testModel(t, filepath.Join(root, "src", "deep"))
	_, cmd := m.openVibe()
	applyEffect(m, cmd)
	if m.vibe.root != root || m.mode != vibeMode {
		t.Fatal("not rooted at repository")
	}
	var hierarchy []string
	for _, r := range m.vibe.rows {
		if r.kind == 'd' || r.kind == 'f' {
			hierarchy = append(hierarchy, r.id)
		}
	}
	for _, id := range []string{"d:", "d:src", "d:src/deep", "f:src/deep/main.go", "d:new", "d:new/deep", "f:new/deep/你好 file.txt"} {
		if !slices.Contains(hierarchy, id) {
			t.Fatalf("missing %s in %v", id, hierarchy)
		}
	}
}

func TestVibeUnbornBinaryLimitsAndQuotedPatch(t *testing.T) {
	root := vibeRepo(t)
	vibeWrite(t, root, "new.txt", "one\ntwo\n")
	vibeTestGit(t, root, "add", "new.txt")
	s := vibeRead(t, root)
	if !strings.Contains(s.branch, "no commits") || vibeFind(t, s, "new.txt").added != 2 {
		t.Fatal("unborn repository")
	}
	vibeCommit(t, root)
	vibeWrite(t, root, "binary.bin", "\x00\x01")
	vibeWrite(t, root, "large.txt", strings.Repeat("x", compareTextLimit+1))
	vibeWrite(t, root, "empty.txt", "")
	s = vibeRead(t, root)
	for _, path := range []string{"binary.bin", "large.txt", "empty.txt"} {
		if vibeFind(t, s, path).note == "" {
			t.Fatalf("missing summary for %s", path)
		}
	}
	files := map[string]*vibeFile{"a\tb.txt": {path: "a\tb.txt"}}
	parseVibePatch([]byte("diff --git \"a/a\\tb.txt\" \"b/a\\tb.txt\"\n--- \"a/a\\tb.txt\"\n+++ \"b/a\\tb.txt\"\n@@ -1 +1 @@\n-old\n+new\n"), files)
	if files["a\tb.txt"].added != 1 {
		t.Fatal("quoted patch filename lost")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readVibe(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestVibeRefreshPreservesPositionAndRejectsStale(t *testing.T) {
	root := vibeRepo(t)
	vibeWrite(t, root, "dir/file.txt", "a\nb\nc\n")
	vibeCommit(t, root)
	vibeWrite(t, root, "dir/file.txt", "a\nchanged\nc\n")
	m := testModel(t, root)
	_, cmd := m.openVibe()
	applyEffect(m, cmd)
	for i, r := range m.vibe.rows {
		if r.kind == 'l' && r.text == "+changed" {
			m.vibe.cursor = i
		}
	}
	oldID := m.vibe.current().id
	applyEffect(m, m.refreshVibe())
	if m.vibe.current().id != oldID {
		t.Fatal("unchanged refresh moved cursor")
	}
	vibeWrite(t, root, "dir/file.txt", "inserted\na\nchanged\nc\n")
	applyEffect(m, m.refreshVibe())
	if m.vibe.current().text != "+changed" {
		t.Fatal("shifted line selection lost")
	}
	first := m.refreshVibe()
	if second := m.refreshVibe(); second != nil || !m.vibe.pending {
		t.Fatal("refreshes not coalesced")
	}
	applyEffect(m, first)
	if m.vibe.loading || m.vibe.pending {
		t.Fatal("queued refresh not drained")
	}
	saved := m.vibe.snapshot.fingerprint
	m.vibe.loading = true
	m.applyVibe(vibeDoneMsg{generation: m.vibe.generation, err: errors.New("temporary failure")})
	if m.vibe.snapshot.fingerprint != saved || m.vibe.err == "" {
		t.Fatal("failure discarded old tree")
	}
	vibeWrite(t, root, "dir/file.txt", "a\nb\nc\n")
	applyEffect(m, m.refreshVibe())
	if m.vibe.current().id != "d:" {
		t.Fatalf("deleted selection didn't fall back to root: %s", m.vibe.current().id)
	}
	pending := m.refreshVibe()
	keyEvent(m, "q")
	m.Update(pending())
	if m.mode != normalMode || len(m.vibe.rows) != 0 {
		t.Fatal("closed Vibe accepted stale result")
	}
}

func TestVibeViewerHistoricalAndUnicode(t *testing.T) {
	root := vibeRepo(t)
	vibeWrite(t, root, "old.go", "old 👩‍💻\n")
	vibeCommit(t, root)
	vibeWrite(t, root, "old.go", "new\n")
	m := testModel(t, root)
	_, cmd := m.openVibe()
	applyEffect(m, cmd)
	for i, r := range m.vibe.rows {
		if r.kind == 'l' && strings.HasPrefix(r.text, "-") {
			m.vibe.cursor = i
			break
		}
	}
	prepare := m.viewVibe()
	ready := prepare().(vibeViewerReadyMsg)
	if ready.err != nil || string(ready.data) != "old 👩‍💻\n" || ready.line != 1 {
		t.Fatalf("snapshot = %#v", ready)
	}
	// Commit the new file after capture: F3 still uses the selected blob.
	vibeCommit(t, root)
	effect := m.startVibeViewer(ready)
	process := effect().(event.ProcessMsg)
	path := process.Command.Args[len(process.Command.Args)-1]
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "old 👩‍💻\n" || filepath.Ext(path) != ".go" {
		t.Fatal("incorrect historical file")
	}
	for _, arg := range []string{"-theme=dracula", "-no-git", "-select=1:1-1:6"} {
		if !slices.Contains(process.Command.Args, arg) {
			t.Fatalf("missing %s in %v", arg, process.Command.Args)
		}
	}
	done := process.Next(errors.New("viewer launch failed"))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("temporary file retained")
	}
	m.Update(done)
	if m.vibe.viewing {
		t.Fatal("viewer did not resume refresh")
	}
	custom := vibeViewerCommand(ToolConfig{Command: "custom-viewer", Type: "path", Args: []string{"--option"}}, "nord", ready, "snapshot.go")
	if !slices.Equal(custom.Args, []string{"custom-viewer", "--option", "snapshot.go"}) {
		t.Fatal("custom viewer arguments changed")
	}
}

func TestVibeRenderingAndInputIsolation(t *testing.T) {
	root := vibeRepo(t)
	vibeWrite(t, root, "src/deep/a.go", "old\n")
	vibeCommit(t, root)
	vibeWrite(t, root, "src/deep/a.go", "new\n")
	m := testModel(t, root)
	_, cmd := m.Update(event.KeyMsg{Name: "v"})
	applyEffect(m, cmd)
	for _, theme := range themeList {
		for _, size := range [][2]int{{40, 8}, {100, 24}} {
			t.Run(fmt.Sprintf("%s/%d", theme.name, size[0]), func(t *testing.T) {
				m.theme = newTheme(theme.name)
				m.screenWidth = size[0]
				m.screenHeight = size[1]
				backend := catatui.NewTestBackend(uint16(size[0]), uint16(size[1]))
				terminal, _ := catatui.NewTerminal(backend)
				if err := terminal.Draw(m.draw); err != nil {
					t.Fatal(err)
				}
				text := backend.Buffer().String()
				for _, want := range []string{"Vibe", "src", "deep", "F3"} {
					if !strings.Contains(text, want) {
						t.Fatalf("missing %q in %s", want, text)
					}
				}
			})
		}
	}
	m.screenHeight = 24
	m.screenWidth = 100
	m.vibe.cursor = 0
	rowCount := len(m.vibe.rows)
	keyEvent(m, "space")
	if len(m.vibe.rows) != rowCount {
		t.Fatal("root should remain expanded")
	}
	keyEvent(m, "c")
	keyEvent(m, "]")
	if m.vibe.current().kind != 'h' {
		t.Fatal("hunk navigation didn't expand tree")
	}
	pane := m.activePane
	for _, key := range []string{"tab", "d", "ctrl+w", "Y", "D"} {
		keyEvent(m, key)
	}
	m.Update(m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseDown, Button: term.MouseButtonLeft, X: 80, Y: 0}))
	if m.activePane != pane || m.mode != vibeMode || !m.hasTabs() {
		t.Fatal("Vibe input escaped to panes")
	}
	keyEvent(m, "w")
	if m.taskView || m.mode != vibeMode || !m.vibe.noWrap {
		t.Fatal("w didn't toggle wrapping")
	}
	keyEvent(m, "w")
	if m.vibe.noWrap {
		t.Fatal("w didn't restore wrapping")
	}
	keyEvent(m, "q")
	if m.mode != normalMode {
		t.Fatal("Vibe didn't close")
	}
}

func TestVibeConflictAndHeadRefresh(t *testing.T) {
	root := vibeRepo(t)
	vibeWrite(t, root, "conflict.txt", "base\n")
	vibeCommit(t, root)
	vibeTestGit(t, root, "checkout", "-b", "other")
	vibeWrite(t, root, "conflict.txt", "other\n")
	vibeCommit(t, root)
	vibeTestGit(t, root, "checkout", "main")
	vibeWrite(t, root, "conflict.txt", "main\n")
	vibeCommit(t, root)
	cmd := exec.Command("git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "merge", "other")
	cmd.Dir = root
	if err := cmd.Run(); err == nil {
		t.Fatal("expected a merge conflict")
	}
	s := vibeRead(t, root)
	f := vibeFind(t, s, "conflict.txt")
	if f.status != "U" || len(f.hunks) != 0 || !strings.Contains(f.note, "conflict") {
		t.Fatalf("conflict = %#v", f)
	}
	vibeWrite(t, root, "conflict.txt", "resolved\n")
	vibeCommit(t, root)
	next := vibeRead(t, root)
	if len(next.files) != 0 || next.head == s.head || next.fingerprint == s.fingerprint {
		t.Fatal("HEAD change not reflected")
	}
}

func TestVibePreservesCollapsedHunkAfterLineShift(t *testing.T) {
	file := vibeFile{path: "src/file", hunks: []vibeHunk{{old: 10, new: 10, header: "@@ -10 +10 @@", lines: []vibeLine{{kind: '-', old: 10, text: "old"}, {kind: '+', new: 10, text: "new"}}}}}
	v := vibeState{root: "repo", collapsed: map[string]bool{}, snapshot: vibeSnapshot{files: []vibeFile{file}}}
	v.rebuild()
	for i, r := range v.rows {
		if r.kind == 'h' {
			v.cursor = i
			break
		}
	}
	id := v.current().id
	v.toggle(10)
	file.hunks[0].old = 11
	file.hunks[0].new = 11
	file.hunks[0].header = "@@ -11 +11 @@"
	v.replace(vibeSnapshot{files: []vibeFile{file}}, 10)
	if v.current().id != id || !v.collapsed[id] {
		t.Fatal("hunk expansion or selection changed after shifting")
	}
}

func TestVibeTrackedSizeAndCommandLimits(t *testing.T) {
	root := vibeRepo(t)
	vibeWrite(t, root, "large.txt", strings.Repeat("a", compareTextLimit+1))
	vibeCommit(t, root)
	vibeWrite(t, root, "large.txt", strings.Repeat("a", compareTextLimit)+"b")
	f := vibeFind(t, vibeRead(t, root), "large.txt")
	if len(f.hunks) > 0 || !strings.Contains(f.note, "5 MiB") {
		t.Fatal("large tracked source was previewed")
	}
	if _, err := vibeGit(context.Background(), root, 1, "rev-parse", "HEAD"); !errors.Is(err, errVibeLimit) {
		t.Fatalf("command limit = %v", err)
	}
	vibeWrite(t, root, "cr.txt", strings.Repeat("x\r", vibeRowLimit+1))
	f = vibeFile{path: "cr.txt"}
	readVibeUntracked(root, &f, vibeRowLimit, vibeOutputLimit)
	if len(f.hunks) > 0 {
		t.Fatal("CR-only file bypassed row limit")
	}
}

func TestVibeUnavailableAndJumpModeBinding(t *testing.T) {
	m := testModel(t, t.TempDir())
	_, cmd := m.Update(event.KeyMsg{Name: "v"})
	applyEffect(m, cmd)
	if m.mode != normalMode || len(m.log) == 0 {
		t.Fatal("non-repository activation")
	}
	root := vibeRepo(t)
	m = testModel(t, root)
	m.cfg.Git = false
	_, cmd = m.openVibe()
	applyEffect(m, cmd)
	if m.mode != normalMode {
		t.Fatal("git=false ignored")
	}
	m.cfg.Git = true
	m.mode = jumpMode
	keyEvent(m, "v")
	if m.mode == vibeMode {
		t.Fatal("Jump letter opened Vibe")
	}
}

func TestVibeWheelScrollKeepsSelectionThroughDrawAndRefresh(t *testing.T) {
	m := testModel(t, t.TempDir())
	m.mode = vibeMode
	m.screenHeight = 8
	file := vibeFile{path: "file.txt", hunks: []vibeHunk{{header: "@@ -0,0 +1,30 @@"}}}
	for i := 0; i < 30; i++ {
		file.hunks[0].lines = append(file.hunks[0].lines, vibeLine{kind: '+', new: i + 1, text: fmt.Sprint(i)})
	}
	m.vibe = vibeState{root: "repo", collapsed: map[string]bool{}, snapshot: vibeSnapshot{files: []vibeFile{file}}}
	m.vibe.rebuild()
	m.Update(event.MouseWheelMsg{Button: event.MouseWheelDown})
	if m.vibe.cursor != 0 || m.vibe.start != 3 {
		t.Fatal("wheel moved selection instead of viewport")
	}
	backend := catatui.NewTestBackend(100, 8)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	if m.vibe.start != 3 {
		t.Fatal("drawing snapped viewport back to cursor")
	}
	m.vibe.replace(m.vibe.snapshot, m.vibeHeight())
	if m.vibe.cursor != 0 || m.vibe.start != 3 {
		t.Fatal("refresh snapped viewport back to cursor")
	}
	m.vibe.scroll(1000, m.vibeHeight())
	if m.vibe.start != len(m.vibe.rows)-m.vibeHeight() {
		t.Fatal("scroll exceeded bottom")
	}
	keyEvent(m, "j")
	if m.vibe.cursor != 1 || m.vibe.start != 1 {
		t.Fatal("keyboard navigation did not reveal selection")
	}
	m.Update(event.MouseWheelMsg{Button: event.MouseWheelUp})
	if m.vibe.start != 0 || m.vibe.cursor != 1 {
		t.Fatal("up scroll changed selection or exceeded top")
	}
}

func TestVibeTreeConnectors(t *testing.T) {
	rows := []vibeRow{
		{id: "root", depth: 0},
		{id: "folder", parent: "root", depth: 1, branch: true},
		{id: "file", parent: "folder", depth: 2, branch: true},
		{id: "hunk", parent: "file", depth: 3, branch: true},
		{id: "line", parent: "hunk", depth: 4},
		{id: "other", parent: "root", depth: 1, branch: true},
	}
	want := []string{"", "├─", "│ └─", "│   └─", "│       ", "└─"}
	if got := vibeTreePrefixes(rows, 20); !slices.Equal(got, want) {
		t.Fatalf("connectors = %q, want %q", got, want)
	}
	// Once the first folder collapses, it still connects to its next sibling.
	if got := vibeTreePrefixes([]vibeRow{rows[0], rows[1], rows[5]}, 20); !slices.Equal(got, []string{"", "├─", "└─"}) {
		t.Fatalf("collapsed connectors = %q", got)
	}
}

func TestVibeWrappedHoverAndClick(t *testing.T) {
	m := testModel(t, t.TempDir())
	m.mode = vibeMode
	m.screenWidth = 40
	m.screenHeight = 12
	text := "wrapped words with 界 and 👩‍💻 " + strings.Repeat("more words ", 12) + "END"
	file := vibeFile{path: "file", hunks: []vibeHunk{{header: "@@ -0,0 +1 @@", lines: []vibeLine{{kind: '+', new: 1, text: text}}}}}
	m.vibe = vibeState{root: "repo", collapsed: map[string]bool{}, snapshot: vibeSnapshot{files: []vibeFile{file}}}
	m.vibe.rebuild()
	m.vibe.layout(40)
	row := len(m.vibe.rows) - 1
	position := m.vibe.rowPosition(row)
	var rebuilt strings.Builder
	for _, segment := range m.vibe.visual {
		if segment.row == row {
			rebuilt.WriteString(segment.text)
			if catatui.StringWidth(segment.prefix+segment.text) > 40 {
				t.Fatal("wrapped line exceeds terminal")
			}
		}
	}
	if !strings.Contains(rebuilt.String(), "END") || !strings.Contains(rebuilt.String(), "👩‍💻") {
		t.Fatal("wrapped content lost")
	}
	m.vibe.start = position
	hover := m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseMove, X: 30, Y: 3})
	m.Update(hover)
	if m.vibe.hover != m.vibe.rows[row].id {
		t.Fatal("continuation hover points at wrong row")
	}
	backend := catatui.NewTestBackend(40, 12)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	if backend.Buffer().CellAt(0, 3).Bg != m.theme.selectionStyle.Native().GetBg() {
		t.Fatal("hover background missing")
	}
	m.Update(event.MouseClickMsg{X: 30, Y: 3, Button: event.MouseLeft})
	if m.vibe.cursor != row {
		t.Fatal("continuation click selected wrong row")
	}
	selected := m.vibe.cursor
	m.Update(event.MouseWheelMsg{Button: event.MouseWheelDown})
	if m.vibe.cursor != selected {
		t.Fatal("wrapped scrolling moved cursor")
	}
	m.Update(m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseMove, X: 1, Y: 0}))
	if m.vibe.hover != "" {
		t.Fatal("hover did not clear outside content")
	}
	wrappedCount := len(m.vibe.visual)
	keyEvent(m, "w")
	if m.taskView || !m.vibe.noWrap || len(m.vibe.visual) != len(m.vibe.rows) {
		t.Fatal("wrap toggle did not produce single-line rows")
	}
	keyEvent(m, "w")
	if m.vibe.noWrap || len(m.vibe.visual) != wrappedCount {
		t.Fatal("wrap toggle did not restore layout")
	}
	m.Update(event.MouseClickMsg{X: 39, Y: 2, Button: event.MouseLeft})
	if !m.vibe.scrollbarDragging || m.vibe.start != 0 {
		t.Fatal("scrollbar top click failed")
	}
	m.Update(m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseDrag, X: 39, Y: 10}))
	if m.vibe.start != len(m.vibe.visual)-m.vibeHeight() || m.vibe.cursor != selected {
		t.Fatal("scrollbar drag moved selection or missed bottom")
	}
	m.Update(m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseUp, X: 39, Y: 10}))
	if m.vibe.scrollbarDragging {
		t.Fatal("scrollbar drag did not end")
	}
}

func TestVibeHunkJumpRevealsContent(t *testing.T) {
	m := testModel(t, t.TempDir())
	m.mode = vibeMode
	m.screenWidth = 40
	m.screenHeight = 8
	file := vibeFile{path: "file", hunks: []vibeHunk{
		{header: "@@ -1 +1 @@", old: 1, new: 1, lines: []vibeLine{{kind: '+', new: 1, text: strings.Repeat("long text ", 20)}}},
		{header: "@@ -20 +20 @@", old: 20, new: 20, lines: []vibeLine{{kind: '+', new: 20, text: "target change"}, {kind: ' ', old: 21, new: 21, text: strings.Repeat("context ", 20)}}},
	}}
	m.vibe = vibeState{root: "repo", collapsed: map[string]bool{}, snapshot: vibeSnapshot{files: []vibeFile{file}}}
	m.vibe.rebuild()
	keyEvent(m, "]")
	keyEvent(m, "]")
	if m.vibe.current().kind != 'h' || m.vibe.current().hunk != 1 || m.vibe.start != m.vibe.rowPosition(m.vibe.cursor) {
		t.Fatal("next hunk not positioned at viewport top")
	}
	backend := catatui.NewTestBackend(40, 8)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(backend.Buffer().String(), "target change") {
		t.Fatal("hunk content not visible after jump")
	}
	keyEvent(m, "[")
	if m.vibe.current().hunk != 0 || m.vibe.start != m.vibe.rowPosition(m.vibe.cursor) {
		t.Fatal("previous hunk not positioned at top")
	}
}

func TestVibeExpandCollapseAll(t *testing.T) {
	m := testModel(t, t.TempDir())
	m.mode = vibeMode
	file := vibeFile{path: "dir/file", hunks: []vibeHunk{{header: "hunk", lines: []vibeLine{{kind: '+', new: 1, text: "change"}}}}}
	m.vibe = vibeState{root: "repo", collapsed: map[string]bool{}, snapshot: vibeSnapshot{files: []vibeFile{file}}}
	m.vibe.rebuild()
	m.vibe.layout(98)
	count := len(m.vibe.rows)
	m.vibe.cursor = count - 1
	keyEvent(m, "c")
	if len(m.vibe.rows) != 2 || m.vibe.cursor != 0 || m.vibe.start != 0 {
		t.Fatal("collapse all did not select root")
	}
	for _, r := range m.vibe.all {
		if r.branch && !m.vibe.collapsed[r.id] {
			t.Fatal("branch not collapsed")
		}
	}
	if m.vibe.rows[0].branch || m.vibe.collapsed["d:"] {
		t.Fatal("repository root is collapsible")
	}
	if !strings.Contains(m.vibe.visual[0].prefix, "┬") {
		t.Fatal("root no longer anchors tree branches")
	}
	keyEvent(m, "e")
	if len(m.vibe.rows) != count || len(m.vibe.collapsed) != 0 || m.vibe.cursor != 0 {
		t.Fatal("expand all failed")
	}
	m.vibe.cursor = count - 1
	selected := m.vibe.current().id
	keyEvent(m, "e")
	if m.vibe.current().id != selected {
		t.Fatal("expand all lost selection")
	}
}
