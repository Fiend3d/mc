package main

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Fiend3d/catatui"
	"mc/internal/event"
)

// porcelain joins records the way git -z does: NUL after every one.
func porcelain(records ...string) []byte {
	return []byte(strings.Join(records, "\x00") + "\x00")
}

func TestParseGitStatusMapsRecordsOntoTheListedDirectory(t *testing.T) {
	root := t.TempDir()
	info := parseGitStatus(porcelain(
		"## main...origin/main [ahead 1, behind 2]",
		" M README.md",
		"?? notes.txt",
		"A  internal/paint/paint.go",
		" D internal/old.go",
		"R  moved.go", "old.go", // a rename carries its original path next
		"!! dist/",
		"UU conflict.go",
	), root, root)

	if info.branch != "main" || info.ahead != 1 || info.behind != 2 {
		t.Fatalf("branch header: %q ahead=%d behind=%d", info.branch, info.ahead, info.behind)
	}
	want := map[string]gitState{
		"README.md":   gitModified,
		"notes.txt":   gitUntracked,
		"moved.go":    gitRenamed,
		"dist":        gitIgnored,
		"conflict.go": gitConflicted,
		// Both records below root/internal roll up onto it, and the more
		// severe of the two is the one that shows.
		"internal": gitDeleted,
	}
	for name, state := range want {
		if got := info.states[filepath.Join(root, name)]; got != state {
			t.Fatalf("%s: state %d, want %d", name, got, state)
		}
	}
	// The second half of the rename record is a path, not a status record.
	if _, ok := info.states[filepath.Join(root, "old.go")]; ok {
		t.Fatal("the original path of a rename was read as its own record")
	}
	if info.added != 2 || info.modified != 3 || info.untracked != 1 {
		t.Fatalf("counts: +%d ~%d ?%d", info.added, info.modified, info.untracked)
	}
}

func TestParseGitStatusOnlyClaimsEntriesOfTheListedDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "src")
	info := parseGitStatus(porcelain(
		"## work",
		" M src/main.go",
		" M src/deep/nested/file.go",
		" M elsewhere/other.go",
		" M top.go",
	), root, dir)

	if got := info.states[filepath.Join(dir, "main.go")]; got != gitModified {
		t.Fatalf("a file of the listed directory is unmarked: %d", got)
	}
	if got := info.states[filepath.Join(dir, "deep")]; got != gitModified {
		t.Fatalf("a deep change did not roll up onto its directory: %d", got)
	}
	if len(info.states) != 2 {
		t.Fatalf("paths outside the listed directory leaked in: %v", info.states)
	}
	// A branch with no upstream still names itself.
	if info.branch != "work" || info.ahead != 0 {
		t.Fatalf("branch without upstream: %q ahead=%d", info.branch, info.ahead)
	}
}

func TestParseGitStatusHeaderForms(t *testing.T) {
	root := t.TempDir()
	for _, c := range []struct {
		header string
		branch string
	}{
		{"## HEAD (no branch)", "HEAD"},
		{"## No commits yet on master", "master"},
		{"## main...origin/main", "main"},
		{"## main...origin/main [ahead 3]", "main"},
	} {
		info := parseGitStatus(porcelain(c.header), root, root)
		if info.branch != c.branch {
			t.Fatalf("%q read as %q, want %q", c.header, info.branch, c.branch)
		}
	}
	if info := parseGitStatus(porcelain("## main...origin/main [ahead 3]"), root, root); info.ahead != 3 || info.behind != 0 {
		t.Fatalf("ahead-only header: ahead=%d behind=%d", info.ahead, info.behind)
	}
}

func TestFindRepoRootWalksUpAndStops(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}
	if got := findRepoRoot(deep); got != "" {
		t.Fatalf("a directory outside any repository found the root %q", got)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if got := findRepoRoot(deep); !samePath(got, root) {
		t.Fatalf("repository root %q, want %q", got, root)
	}
	// A worktree or submodule has a .git file rather than a directory.
	file := filepath.Join(t.TempDir(), "linked")
	if err := os.MkdirAll(file, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(file, ".git"), []byte("gitdir: elsewhere"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := findRepoRoot(file); !samePath(got, file) {
		t.Fatalf("worktree root %q, want %q", got, file)
	}
}

// gitTabModel is a model whose current tab is read and ready for a status
// message, with the tab's directory holding the given names.
func gitTabModel(t *testing.T, names ...string) (*model, *tab) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		touch(t, filepath.Join(dir, name))
	}
	m := testModel(t, dir)
	applyEffect(m, m.readTab(m.getTab()))
	return m, m.getTab()
}

func TestGitStatusAppliesToItemsAndIgnoresStaleResults(t *testing.T) {
	m, tab := gitTabModel(t, "tracked.go", "fresh.go")
	tab.gitGeneration = 7
	info := &gitInfo{branch: "main", modified: 1, states: map[string]gitState{
		filepath.Join(tab.dir, "tracked.go"): gitModified,
	}}
	m.Update(gitStatusMsg{target: tab, generation: 7, page: tab.page, dir: tab.dir, info: info})

	fresh, tracked := tab.page.getItems()[0].(*filepathItem), tab.page.getItems()[1].(*filepathItem)
	if tracked.git != gitModified {
		t.Fatalf("the status did not reach the item: %d", tracked.git)
	}
	if fresh.git != gitNone {
		t.Fatalf("an unmentioned file was marked: %d", fresh.git)
	}
	if tab.git == nil || tab.git.branch != "main" {
		t.Fatal("the tab did not keep the repository information")
	}

	// Anything that arrives for a generation the tab has moved past, or for a
	// page it has replaced, is dropped rather than written over fresh state.
	stale := &gitInfo{states: map[string]gitState{filepath.Join(tab.dir, "fresh.go"): gitUntracked}}
	m.Update(gitStatusMsg{target: tab, generation: 6, page: tab.page, dir: tab.dir, info: stale})
	m.Update(gitStatusMsg{target: tab, generation: 7, page: &page{}, dir: tab.dir, info: stale})
	if fresh.git != gitNone {
		t.Fatalf("a stale result was applied: %d", fresh.git)
	}
}

func TestGitStateSurvivesARefreshAndDoesNotChurnTheListing(t *testing.T) {
	m, tab := gitTabModel(t, "one.go")
	items := tab.page.getItems()
	items[0].(*filepathItem).git = gitModified

	// A marker is calculated data, like a directory size: it must not make an
	// otherwise identical listing look changed.
	fresh, err := readItems(tab.dir)
	if err != nil {
		t.Fatal(err)
	}
	if !unchangedListing(tab.page.items, fresh) {
		t.Fatal("a git marker made an unchanged listing look changed")
	}

	// An explicit refresh does rebuild the listing, and must carry the markers
	// over so the column does not blank while git is asked again.
	applyEffect(m, m.readTab(tab))
	if got := tab.page.getItems()[0].(*filepathItem).git; got != gitModified {
		t.Fatalf("the marker was lost across a refresh: %d", got)
	}
}

func TestPaneDrawsTheGitColumnAndSummary(t *testing.T) {
	m, tab := gitTabModel(t, "changed.go", "plain.go")
	backend := catatui.NewTestBackend(100, 24)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	before := strings.Split(backend.Buffer().String(), "\n")[2]

	tab.git = &gitInfo{branch: "main", ahead: 1, modified: 1, untracked: 2, states: map[string]gitState{}}
	tab.page.getItems()[0].(*filepathItem).git = gitModified
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(backend.Buffer().String(), "\n")

	if !strings.Contains(lines[2], "M changed.go") {
		t.Fatalf("no marker column beside the changed file: %q", lines[2])
	}
	if !strings.Contains(lines[3], "  plain.go") {
		t.Fatalf("an unchanged file lost its alignment: %q", lines[3])
	}
	// The column costs the name one cell, so the metadata stays where it was.
	if strings.Index(before, "1B") != strings.Index(lines[2], "1B") {
		t.Fatalf("the metadata column moved:\n%q\n%q", before, lines[2])
	}
	// The summary belongs to the path row, at the top where the eye goes, and
	// the status row keeps its counts to itself.
	if !strings.Contains(lines[1], "main↑1 ~1 ?2") {
		t.Fatalf("path row without the branch summary: %q", lines[1])
	}
	status := ""
	for _, line := range lines {
		if strings.Contains(line, "items ·") {
			status = line
		}
	}
	if strings.Contains(status, "main") {
		t.Fatalf("the summary is still on the status row as well: %q", status)
	}
}

// themeHex names a theme colour the way the test backend names a cell's.
func themeHex(c color.Color) string {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return "default"
	}
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// cellForeground names a cell's foreground colour, as cellBackground does for
// the other side.
func cellForeground(c *catatui.Cell) string {
	r, g, b, ok := c.Fg.RGB()
	if !ok {
		return "default"
	}
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// pathRowColor reads the colour the given text of the path row was drawn in.
func pathRowColor(t *testing.T, backend *catatui.TestBackend, text string) string {
	t.Helper()
	row := strings.Split(backend.Buffer().String(), "\n")[1]
	x := strings.Index(row, text)
	if x < 0 {
		t.Fatalf("%q is not on the path row: %q", text, row)
	}
	return cellForeground(backend.Buffer().CellAt(uint16(x), 1))
}

func TestPathRowMarksTheRepositoryRoot(t *testing.T) {
	m, tab := gitTabModel(t, "one.go")
	// A short path of its own: a temp directory is long enough to be truncated
	// on the row, and the point here is which component is coloured. The tab
	// sits below the root, so the root is a component in the middle of the
	// path rather than its last one.
	root := `C:\work\repo`
	tab.dir = filepath.Join(root, "inside")
	tab.git = &gitInfo{root: root, branch: "main", modified: 1, states: map[string]gitState{}}

	backend := catatui.NewTestBackend(100, 24)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	dirty := pathRowColor(t, backend, filepath.Base(root))
	if plain := pathRowColor(t, backend, "inside"); dirty == plain {
		t.Fatalf("the repository root is drawn like the rest of the path: %s", dirty)
	}

	// Committing everything turns the mark green without moving it.
	tab.git = &gitInfo{root: root, branch: "main", states: map[string]gitState{}}
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	clean := pathRowColor(t, backend, filepath.Base(root))
	if clean == dirty {
		t.Fatalf("a clean work tree reads the same as a dirty one: %s", clean)
	}
	if clean != themeHex(m.theme.greenColor) {
		t.Fatalf("a clean work tree is not green: %s", clean)
	}
}

func TestPathRowKeepsThePathWhenTheLaneIsNarrow(t *testing.T) {
	_, tab := gitTabModel(t, "one.go")
	tab.git = &gitInfo{root: tab.dir, branch: "a-rather-long-branch-name", modified: 3, states: map[string]gitState{}}

	if width, summary := gitPathLane(tab, 30, false); width != 30 || summary != "" {
		t.Fatalf("a narrow pane gave the path away: width %d, summary %q", width, summary)
	}
	width, summary := gitPathLane(tab, 100, false)
	if summary == "" || width >= 100 {
		t.Fatalf("a wide pane did not make room for the summary: width %d, summary %q", width, summary)
	}
	// Path mode types into that row, so the summary steps aside for it.
	if width, summary := gitPathLane(tab, 100, true); width != 100 || summary != "" {
		t.Fatalf("the summary stayed while the path was being edited: width %d, summary %q", width, summary)
	}
}

func TestBreadcrumbClicksIgnoreTheGitLane(t *testing.T) {
	m, tab := gitTabModel(t, "one.go")
	tab.dir = `C:\work\repo\inside`
	tab.git = &gitInfo{root: `C:\work\repo`, branch: "main", modified: 1, states: map[string]gitState{}}
	pathWidth, summary := gitPathLane(tab, m.width, false)
	if summary == "" {
		t.Fatalf("this test needs the summary on the row: pane width %d", m.width)
	}

	// A click in the summary's lane belongs to no path component and must not
	// navigate anywhere.
	_, cmd := m.Update(event.MouseClickMsg{X: pathWidth + 1, Y: 0, Button: event.MouseLeft})
	if cmd != nil || tab.dir != `C:\work\repo\inside` {
		t.Fatalf("clicking the summary navigated to %q", tab.dir)
	}
	// A component still takes the click: "repo" ends at column 12.
	m.click = mouseClick{}
	if _, cmd := m.Update(event.MouseClickMsg{X: 11, Y: 0, Button: event.MouseLeft}); cmd == nil {
		t.Fatal("clicking a path component no longer navigates")
	}
	if tab.dir != `C:\work\repo` {
		t.Fatalf("clicked component led to %q", tab.dir)
	}
}

// TestGitStatusLeavesTheRepositoryAlone guards the loop that --no-optional-locks
// closes: a status that refreshes the index on disk bumps .git's timestamp, mc
// sees its own tab directory change, re-reads it and asks git again.
func TestGitStatusLeavesTheRepositoryAlone(t *testing.T) {
	if gitBinary() == "" {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "mc@example.com"}, {"config", "user.name", "mc"}} {
		cmd := exec.Command(gitBinary(), args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	tracked := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-qm", "first"}} {
		cmd := exec.Command(gitBinary(), args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	// A file whose timestamp moved is what makes git want to write the index
	// back out, which is the case that used to churn.
	now := time.Now().Add(time.Minute)
	if err := os.Chtimes(tracked, now, now); err != nil {
		t.Fatal(err)
	}

	git := filepath.Join(dir, ".git")
	before, err := os.Stat(git)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readGitInfo(context.Background(), dir, dir); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(git)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("reading the status wrote to .git, which feeds mc's own refresh back into it")
	}
}

// TestPathHoverSurvivesARefresh keeps the breadcrumb highlight from blinking
// out from under a resting pointer whenever the listing reloads.
func TestPathHoverSurvivesARefresh(t *testing.T) {
	m, tab := gitTabModel(t, "one.go")
	m.Update(event.MouseHoverMsg{Pane: 0, Index: -1, Path: true, X: 7})
	m.Update(event.MouseHoverMsg{Pane: 0, Index: 3})
	if m.hoverPathX != -1 {
		t.Fatal("hovering a row left the path hover in place")
	}

	m.Update(event.MouseHoverMsg{Pane: 0, Index: -1, Path: true, X: 7})
	touch(t, filepath.Join(tab.dir, "two.go"))
	applyEffect(m, m.readTabMode(tab, true))

	if m.hoverPathX != 7 || m.hoverPathPane != 0 {
		t.Fatalf("a refresh took the breadcrumb highlight away: pane %d x %d", m.hoverPathPane, m.hoverPathX)
	}
	// The row and tab hovers are indices into what just changed, so those do go.
	if m.hoverIndex != -1 {
		t.Fatalf("a refresh kept a row hover pointing at index %d", m.hoverIndex)
	}
}

// TestGitStatusReadsARealRepository checks the flags against the installed
// git rather than against a fixture, so a change in its output is caught here.
func TestGitStatusReadsARealRepository(t *testing.T) {
	if gitBinary() == "" {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "mc@example.com"}, {"config", "user.name", "mc"}} {
		cmd := exec.Command(gitBinary(), args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	touch(t, filepath.Join(dir, "untracked.txt"))
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"), 0644); err != nil {
		t.Fatal(err)
	}
	touch(t, filepath.Join(dir, "ignored.txt"))

	info, err := readGitInfo(context.Background(), dir, dir)
	if err != nil {
		t.Fatalf("reading a fresh repository failed: %v", err)
	}
	if got := info.states[filepath.Join(dir, "untracked.txt")]; got != gitUntracked {
		t.Fatalf("untracked file read as %d", got)
	}
	if got := info.states[filepath.Join(dir, "ignored.txt")]; got != gitIgnored {
		t.Fatalf("ignored file read as %d", got)
	}
	if info.branch == "" {
		t.Fatal("no branch read from a fresh repository")
	}
}

// TestGitStatusStaysOutOfTheWayOutsideRepositories proves the feature costs
// nothing where it does not apply: no command, no column, no branch text.
func TestGitStatusStaysOutOfTheWayOutsideRepositories(t *testing.T) {
	m, tab := gitTabModel(t, "file.go")
	if cmd := m.readGitStatus(tab); cmd != nil {
		t.Fatal("a directory outside any repository still asked git")
	}
	tab.git = &gitInfo{branch: "main", states: map[string]gitState{}}
	m.cfg.Git = false
	if cmd := m.readGitStatus(tab); cmd != nil || tab.git != nil {
		t.Fatal("the config flag did not turn the feature off")
	}
	var _ event.Cmd = m.readGitStatus(tab)
}
