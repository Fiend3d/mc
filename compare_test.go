package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Fiend3d/catatui"
	"github.com/Fiend3d/catatui/term"
	"github.com/charmbracelet/x/ansi"
	"mc/internal/event"
)

func comparisonFixture(t *testing.T, left, right string) (*model, [2]string) {
	t.Helper()
	dirs := [2]string{t.TempDir(), t.TempDir()}
	var paths [2]string
	m := testModel(t, dirs[:]...)
	for side, data := range []string{left, right} {
		paths[side] = filepath.Join(dirs[side], "example.txt")
		if err := os.WriteFile(paths[side], []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		m.panes[side].tabs[0].page.items = []item{&filepathItem{name: "example.txt", fullPath: paths[side]}}
	}
	return m, paths
}

func TestCompareMyersShortestScript(t *testing.T) {
	// Compare against an independent dynamic-programming distance oracle and
	// reconstruct both inputs, including repeated lines and empty sequences.
	rng := rand.New(rand.NewSource(42))
	for trial := 0; trial < 1000; trial++ {
		a, b := make([]byte, rng.Intn(22)), make([]byte, rng.Intn(22))
		for i := range a {
			a[i] = byte(rng.Intn(4))
		}
		for i := range b {
			b[i] = byte(rng.Intn(4))
		}
		budget := compareWorkLimit
		ops, err := compareEdits(context.Background(), len(a), len(b), func(i, j int) bool { return a[i] == b[j] }, &budget)
		if err != nil {
			t.Fatal(err)
		}
		i, j, cost := 0, 0, 0
		for _, op := range ops {
			switch op {
			case '=':
				if i >= len(a) || j >= len(b) || a[i] != b[j] {
					t.Fatalf("invalid match: %v %v %s", a, b, ops)
				}
				i++
				j++
			case '-':
				i++
				cost++
			case '+':
				j++
				cost++
			}
		}
		if i != len(a) || j != len(b) {
			t.Fatal("script did not consume inputs")
		}
		dp := make([][]int, len(a)+1)
		for i := range dp {
			dp[i] = make([]int, len(b)+1)
			dp[i][0] = i
		}
		for j := range dp[0] {
			dp[0][j] = j
		}
		for i := 1; i <= len(a); i++ {
			for j := 1; j <= len(b); j++ {
				if a[i-1] == b[j-1] {
					dp[i][j] = dp[i-1][j-1]
				} else {
					dp[i][j] = min(dp[i-1][j], dp[i][j-1]) + 1
				}
			}
		}
		if cost != dp[len(a)][len(b)] {
			t.Fatalf("nonminimal cost %d vs %d", cost, dp[len(a)][len(b)])
		}
	}
}

func TestCompareTextAndSummaries(t *testing.T) {
	for _, tc := range []struct {
		name, left, right string
		equal             bool
		hunks             int
		note              string
	}{
		{"empty", "", "", true, 0, ""},
		{"identical", "same\n", "same\n", true, 0, ""},
		{"insert", "a\nc\n", "a\nb\nc\n", false, 1, ""},
		{"delete", "a\nb\nc\n", "a\nc\n", false, 1, ""},
		{"replace", "a\nold\nz\n", "a\nnew\nz\n", false, 1, ""},
		{"two hunks", "a\nb\nc\nd\n", "x\nb\nc\ny\n", false, 2, ""},
		{"newline", "a\r\n", "a\n", false, 1, ""},
		{"missing newline", "a\n", "a", false, 1, ""},
		{"whitespace", " a\n", "a\n", false, 1, ""},
		{"unicode", "你好 👩‍💻\n", "你好 👩‍🔬\n", false, 1, ""},
		{"bom", "\ufeffa\n", "a\n", false, 0, "BOM differs"},
		{"binary same", "a\x00b", "a\x00b", true, 0, "Binary"},
		{"binary different", "a\x00b", "a\x00c", false, 0, "Binary"},
		{"encoding", "\xff", "\xfe", false, 0, "unsupported"},
		{"large", strings.Repeat("x", compareTextLimit+1), "x", false, 0, "5 MiB"},
		{"complex", strings.Repeat("a\n", 2000), strings.Repeat("b\n", 2000), false, 0, "work limit"},
		{"many lines", strings.Repeat("\n", 100_001), "", false, 0, "line limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, paths := comparisonFixture(t, tc.left, tc.right)
			r := compareFiles(context.Background(), paths)
			if r.err != nil || r.equal != tc.equal || len(r.hunks) != tc.hunks || !strings.Contains(r.note, tc.note) {
				t.Fatalf("result equal=%v hunks=%v note=%s err=%v", r.equal, r.hunks, r.note, r.err)
			}
			if r.sizes != [2]int64{int64(len(tc.left)), int64(len(tc.right))} {
				t.Fatal("wrong sizes")
			}
			for side := 0; side < 2; side++ {
				var rebuilt strings.Builder
				for _, row := range r.rows {
					number := row.left
					if side == 1 {
						number = row.right
					}
					if number > 0 {
						l := r.lines[side][number-1]
						rebuilt.WriteString(l.text + l.ending)
					}
				}
				if len(r.rows) > 0 && rebuilt.String() != strings.TrimPrefix([]string{tc.left, tc.right}[side], "\ufeff") {
					t.Fatal("alignment lost source text")
				}
			}
		})
	}
}

func TestCompareInlineAndLiteralClipping(t *testing.T) {
	budget := compareWorkLimit
	a, b := compareInline(context.Background(), "hello red world", "hello green world", &budget)
	if a[0] || b[len(b)-1] || !a[8] || !b[6] {
		t.Fatalf("inline marks %v %v", a, b)
	}
	m := testModel(t, t.TempDir())
	style := m.theme.baseStyle
	for _, tc := range []struct {
		text          string
		offset, width int
		want          string
	}{
		{"a\tb", 0, 5, "a   b"},
		{"界a", 1, 2, " a"},
		{"e\u0301界", 0, 1, "e\u0301"},
		{"\x1b[31m", 0, 20, "\\u001B[31m"},
	} {
		got := ansi.Strip(compareLineText(compareLine{text: tc.text}, nil, tc.offset, tc.width, style, style, false))
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.text, got, tc.want)
		}
	}
}

func TestCompareCursorIsolationReloadAndCancellation(t *testing.T) {
	m, paths := comparisonFixture(t, "one\n", "two\n")
	for _, p := range m.panes {
		tab := p.tabs[0]
		tab.page.items = append([]item{&filepathItem{name: "marked", fullPath: "missing", selected: true}}, tab.page.items...)
		// Filtering must resolve the visible cursor, not the backing row.
		tab.page.tempItems = tab.page.items[1:]
	}
	m.activePane = 1
	m.pane = m.panes[1]
	_, cmd := m.Update(m.inputEvent(term.Event{Kind: term.EventKey, Key: term.KeyRune, Rune: 'D', Mods: term.ModShift}))
	if m.mode != compareMode || m.compare.paths != paths || cmd == nil {
		t.Fatal("cursor paths not captured")
	}
	old := cmd()
	_, reload := m.Update(event.KeyMsg{Name: "f5"})
	m.Update(old)
	if !m.compare.loading {
		t.Fatal("stale result applied")
	}
	applyEffect(m, reload)
	if m.compare.loading || len(m.compare.result.hunks) != 1 {
		t.Fatal("comparison not loaded")
	}
	for _, key := range []string{"tab", "D", "d", "ctrl+w", "Y", "f4"} {
		keyEvent(m, key)
	}
	m.Update(m.inputEvent(term.Event{Kind: term.EventMouse, MouseKind: term.MouseDown, Button: term.MouseButtonLeft, X: 2, Y: 0}))
	if m.activePane != 1 || len(m.panes[0].tabs) != 1 || len(m.panes[1].tabs) != 1 {
		t.Fatal("compare input reached panes")
	}
	keyEvent(m, "w")
	keyEvent(m, "esc")
	if m.taskView || m.mode != compareMode {
		t.Fatal("task overlay did not return to Compare")
	}
	if err := os.WriteFile(paths[1], []byte("one\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, reload = m.Update(event.KeyMsg{Name: "f5"})
	applyEffect(m, reload)
	if !m.compare.result.equal {
		t.Fatal("F5 did not read modified file")
	}
	_, pending := m.Update(event.KeyMsg{Name: "f5"})
	keyEvent(m, "esc")
	m.Update(pending())
	if m.mode != normalMode || len(m.compare.result.rows) != 0 {
		t.Fatal("closed comparison accepted result")
	}
	for _, p := range m.panes {
		if !p.tabs[0].page.items[0].isSelected() || p.tabs[0].getPageSettings().cursor != 0 {
			t.Fatal("pane state changed")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(compareFiles(ctx, paths).err, context.Canceled) {
		t.Fatal("read ignored cancellation")
	}
	budget := compareWorkLimit
	if _, err := compareEdits(ctx, 1, 1, func(i, j int) bool { return true }, &budget); !errors.Is(err, context.Canceled) {
		t.Fatal("diff ignored cancellation")
	}
}

func TestCompareInvalidTargets(t *testing.T) {
	for _, kind := range []string{"empty pane", "empty list", "directory", "missing file"} {
		t.Run(kind, func(t *testing.T) {
			m, paths := comparisonFixture(t, "a", "b")
			switch kind {
			case "empty pane":
				m.panes[0].tabs = nil
			case "empty list":
				m.panes[1].tabs[0].page.items = nil
			case "directory":
				m.panes[1].tabs[0].page.items[0].(*filepathItem).isDir = true
			case "missing file":
				if err := os.Remove(paths[1]); err != nil {
					t.Fatal(err)
				}
			}
			_, cmd := m.Update(event.KeyMsg{Name: "D"})
			applyEffect(m, cmd)
			if kind == "missing file" {
				if m.compare.result.err == nil {
					t.Fatal("missing read error")
				}
			} else if m.mode != normalMode || len(m.log) == 0 {
				t.Fatal("missing target warning")
			}
		})
	}
}

func TestCompareRenderingThemesAndNavigation(t *testing.T) {
	m, _ := comparisonFixture(t, "old\n"+strings.Repeat("same\n", 20)+"last\n", "new\n"+strings.Repeat("same\n", 20)+"final\n")
	_, cmd := m.openCompare()
	applyEffect(m, cmd)
	keyEvent(m, "n")
	if m.compare.start != 21 {
		t.Fatal("next hunk")
	}
	keyEvent(m, "n")
	if m.compare.start != 21 {
		t.Fatal("next should stop")
	}
	keyEvent(m, "down")
	if m.compare.start != 21 {
		t.Fatal("scroll down moved backwards from final hunk")
	}
	keyEvent(m, "p")
	if m.compare.start != 0 {
		t.Fatal("previous hunk")
	}
	for _, preset := range themeList {
		for _, size := range [][2]int{{40, 8}, {100, 24}, {160, 40}} {
			t.Run(fmt.Sprintf("%s/%d", preset.name, size[0]), func(t *testing.T) {
				m.theme = newTheme(preset.name)
				m.screenWidth = size[0]
				m.screenHeight = size[1]
				m.dimensions()
				backend := catatui.NewTestBackend(uint16(size[0]), uint16(size[1]))
				terminal, _ := catatui.NewTerminal(backend)
				if err := terminal.Draw(m.draw); err != nil {
					t.Fatal(err)
				}
				buf := backend.Buffer()
				text := buf.String()
				for _, want := range []string{"Differences", "LEFT", "RIGHT", "old", "new", "Esc close"} {
					if !strings.Contains(text, want) {
						t.Fatalf("missing %q:\n%s", want, text)
					}
				}
				leftBg := buf.CellAt(0, 3).Bg
				rightBg := buf.CellAt(uint16((size[0]-1)/2+1), 3).Bg
				if reflect.DeepEqual(leftBg, rightBg) || reflect.DeepEqual(leftBg, m.theme.baseStyle.Native().GetBg()) {
					t.Fatal("missing change backgrounds")
				}
			})
		}
	}
}
