package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fiend3d/catatui"
	"mc/internal/event"
)

// gitRepoModel is a model whose tab holds a repository with one changed file
// in a subdirectory, so a jump has somewhere to go.
func gitRepoModel(t *testing.T) (*model, *tab, string) {
	t.Helper()
	dir := t.TempDir()
	deep := filepath.Join(dir, "internal")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}
	buried := filepath.Join(deep, "buried.go")
	touch(t, buried)
	touch(t, filepath.Join(dir, "top.go"))

	m := testModel(t, dir)
	applyEffect(m, m.readTab(m.getTab()))
	tab := m.getTab()
	tab.git = &gitInfo{
		root: dir, branch: "main", modified: 2, untracked: 1,
		states: map[string]gitState{},
		entries: []gitEntry{
			{filepath.Join(dir, "top.go"), gitModified},
			{buried, gitDeleted},
			{filepath.Join(dir, "fresh.go"), gitUntracked},
		},
	}
	return m, tab, buried
}

func TestParseGitStatusKeepsEveryChangedFile(t *testing.T) {
	root := t.TempDir()
	info := parseGitStatus(porcelain(
		"## main",
		" M top.go",
		" D internal/gone.go",
		"?? notes.txt",
		"A  internal/paint/new.go",
		"!! dist/",
	), root, root)

	// The entries are repository-wide, unlike the states map, which only
	// covers what the listed directory shows.
	if len(info.entries) != 4 {
		t.Fatalf("entries %v", info.entries)
	}
	if info.entries[1].path != filepath.Join(root, "internal", "gone.go") {
		t.Fatalf("a path outside the listed directory was not kept: %q", info.entries[1].path)
	}
	for _, e := range info.entries {
		if e.state == gitIgnored {
			t.Fatal("an ignored entry reached a list, but no tally counts it")
		}
	}
	// A list opened from a tally must hold exactly as many rows as the tally.
	for _, c := range []struct {
		tally gitState
		count int
	}{{gitModified, info.modified}, {gitUntracked, info.untracked}, {gitAdded, info.added}} {
		if got := len(info.changed(c.tally)); got != c.count {
			t.Fatalf("tally %d says %d, list has %d", c.tally, c.count, got)
		}
	}
}

func TestGitSummaryHitTesting(t *testing.T) {
	_, tab, _ := gitRepoModel(t)
	const width = 60
	summary := tab.git.summary() // "main ~2 ?1"
	start := gitSummaryStart(tab, width)

	if got := gitSummaryAtX(tab, width, start+1); got != gitNone {
		t.Fatalf("the branch is not a target, but x gave %d", got)
	}
	if got := gitSummaryAtX(tab, width, start-2); got != gitNone {
		t.Fatalf("the path side of the row gave %d", got)
	}
	if got := gitSummaryAtX(tab, width, start+strings.Index(summary, "~")); got != gitModified {
		t.Fatalf("the modified tally gave %d", got)
	}
	if got := gitSummaryAtX(tab, width, start+strings.Index(summary, "?")); got != gitUntracked {
		t.Fatalf("the untracked tally gave %d", got)
	}
	// A pane too narrow for the summary has no targets on its path row.
	if got := gitSummaryAtX(tab, 30, 25); got != gitNone {
		t.Fatalf("a row without a summary still answered %d", got)
	}
}

func TestGitSummaryHighlightsOnlyTheHoveredTally(t *testing.T) {
	m, tab, _ := gitRepoModel(t)
	backend := catatui.NewTestBackend(100, 24)
	terminal, _ := catatui.NewTerminal(backend)

	marks := func() (foreground, background []string) {
		if err := terminal.Draw(m.draw); err != nil {
			t.Fatal(err)
		}
		for x := 0; x < m.width; x++ {
			cell := backend.Buffer().CellAt(uint16(x), 1)
			foreground = append(foreground, cellForeground(cell))
			background = append(background, cellBackground(*cell))
		}
		return foreground, background
	}
	plainFg, plainBg := marks()

	x := gitSummaryStart(tab, m.width) + strings.Index(tab.git.summary(), "~")
	m.Update(event.MouseHoverMsg{Pane: 0, Index: -1, Path: true, X: x})
	hoveredFg, hoveredBg := marks()

	// A tally lights up the way a breadcrumb component does: the text turns
	// white and the row behind it does not move at all.
	changed := 0
	for i := range plainFg {
		if plainFg[i] != hoveredFg[i] {
			changed++
			if hoveredFg[i] != themeHex(m.theme.whiteColor) {
				t.Fatalf("the hovered tally turned %s, not the plain white the path uses", hoveredFg[i])
			}
		}
	}
	if changed != 3 {
		t.Fatalf("%d cells lit up, want the 3 of the tally", changed)
	}
	for i := range plainBg {
		if plainBg[i] != hoveredBg[i] {
			t.Fatalf("the background moved at column %d: %s then %s", i, plainBg[i], hoveredBg[i])
		}
	}
}

func TestClickingATallyOpensItsList(t *testing.T) {
	m, tab, _ := gitRepoModel(t)
	summary := tab.git.summary()
	start := gitSummaryStart(tab, m.width)

	// The branch is not a target, so a click there leaves the mode alone.
	m.Update(event.MouseClickMsg{X: start + 1, Y: 0, Button: event.MouseLeft})
	if m.mode != normalMode {
		t.Fatalf("clicking the branch opened mode %d", m.mode)
	}

	m.click = mouseClick{}
	m.Update(event.MouseClickMsg{X: start + strings.Index(summary, "~"), Y: 0, Button: event.MouseLeft})
	if m.mode != gitListMode {
		t.Fatalf("clicking a tally left the mode at %d", m.mode)
	}
	if len(m.gitList.entries) != tab.git.modified {
		t.Fatalf("the list holds %d rows, the tally said %d", len(m.gitList.entries), tab.git.modified)
	}
	if m.gitList.tally != gitModified {
		t.Fatalf("the list was opened for tally %d", m.gitList.tally)
	}
}

func TestGitListJumpsToTheEntryAndToolsActOnIt(t *testing.T) {
	m, tab, buried := gitRepoModel(t)
	if _, cmd := m.openGitList(gitModified); cmd != nil {
		applyEffect(m, cmd)
	}
	keyEvent(m, "j") // onto the file buried in the subdirectory

	// An F-key tool is handed the highlighted entry, the way search mode hands
	// over its own.
	if paths := m.getPaths(); len(paths) != 1 || paths[0] != buried {
		t.Fatalf("a tool would have acted on %v", paths)
	}

	_, cmd := m.Update(event.KeyMsg{Name: "enter"})
	applyEffect(m, cmd)
	if m.mode != normalMode {
		t.Fatalf("the list stayed open after a jump: mode %d", m.mode)
	}
	if !samePath(tab.dir, filepath.Dir(buried)) {
		t.Fatalf("the pane went to %q, want %q", tab.dir, filepath.Dir(buried))
	}
	settings := tab.getPageSettings()
	items := tab.page.getItems()
	if len(items) == 0 || items[settings.cursor].getFullPath() != buried {
		t.Fatalf("the cursor did not land on the file: %v", items)
	}
}

func TestGitListEscapeLeavesEverythingWhereItWas(t *testing.T) {
	m, tab, buried := gitRepoModel(t)
	before := tab.dir
	m.openGitList(gitModified)
	keyEvent(m, "j")
	keyEvent(m, "esc")
	if m.mode != normalMode || tab.dir != before {
		t.Fatalf("Esc left mode %d and directory %q", m.mode, tab.dir)
	}
	// Back in the pane, a tool acts on the pane's own cursor again, not on
	// whatever the closed list was pointing at.
	if paths := m.getPaths(); len(paths) == 1 && paths[0] == buried {
		t.Fatal("the closed list still decides what a tool would act on")
	}
}

func TestGitListRefusesWhenThereIsNothingToShow(t *testing.T) {
	m, tab, _ := gitRepoModel(t)
	logs := len(m.log)
	if _, cmd := m.openGitList(gitAdded); cmd != nil {
		applyEffect(m, cmd)
	}
	if m.mode != normalMode {
		t.Fatalf("an empty tally opened mode %d", m.mode)
	}
	if len(m.log) == logs {
		t.Fatal("an empty tally said nothing at all")
	}

	tab.git = nil
	logs = len(m.log)
	if _, cmd := m.openGitList(gitModified); cmd != nil {
		applyEffect(m, cmd)
	}
	if m.mode != normalMode || len(m.log) == logs {
		t.Fatalf("outside a repository the list opened anyway: mode %d", m.mode)
	}
}

func TestGoModeOpensTheLists(t *testing.T) {
	for _, c := range []struct {
		key   string
		tally gitState
	}{{"m", gitModified}, {"u", gitUntracked}} {
		m, _, _ := gitRepoModel(t)
		keyEvent(m, "g")
		keyEvent(m, c.key)
		if m.mode != gitListMode || m.gitList.tally != c.tally {
			t.Fatalf("g%s opened mode %d for tally %d", c.key, m.mode, m.gitList.tally)
		}
	}
}
