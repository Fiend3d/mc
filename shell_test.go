package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mc/internal/event"
)

// collectMsgs flattens a command's messages without dispatching them, so a
// scheduled process can be inspected instead of run by the harness.
func collectMsgs(c event.Cmd) []event.Msg {
	if c == nil {
		return nil
	}
	switch v := c().(type) {
	case event.BatchMsg:
		var out []event.Msg
		for _, sub := range v {
			out = append(out, collectMsgs(sub)...)
		}
		return out
	case event.SequenceMsg:
		var out []event.Msg
		for _, sub := range v {
			out = append(out, collectMsgs(sub)...)
		}
		return out
	case nil:
		return nil
	default:
		return []event.Msg{v}
	}
}

// lastMessage returns the text of the most recent log entry.
func lastMessage(t *testing.T, m *model) string {
	t.Helper()
	if len(m.log) == 0 {
		t.Fatal("no messages logged")
	}
	return m.log[len(m.log)-1].message
}

func TestShellArgsExpandsTheSelectionMacro(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "plain.txt"))
	touch(t, filepath.Join(dir, "with space.txt"))
	m := testModel(t, dir)
	applyEffect(m, m.readTab(m.getTab()))
	m.sort(alphabeticSort, false)
	for _, it := range m.getPage().getItems() {
		it.setSelected(true)
	}

	args := m.shellArgs("echo #sl end")
	if len(args) != 4 {
		t.Fatalf("expected the macro to expand to both paths, got %q", args)
	}
	if args[0] != "echo" || args[3] != "end" {
		t.Fatalf("surrounding tokens were disturbed: %q", args)
	}
	// A path containing a space must survive being joined back into one line.
	quoted := 0
	for _, a := range args[1:3] {
		if strings.HasPrefix(a, `"`) && strings.HasSuffix(a, `"`) {
			quoted++
		}
	}
	if quoted != 1 {
		t.Fatalf("expected exactly the spaced path to be quoted: %q", args)
	}
}

func TestShellArgsWithoutMacroIsUnchanged(t *testing.T) {
	m := testModel(t, t.TempDir())
	args := m.shellArgs("git status --short")
	if strings.Join(args, " ") != "git status --short" {
		t.Fatalf("command line was rewritten: %q", args)
	}
}

func TestRepeatShellWithoutHistoryReportsIt(t *testing.T) {
	m := testModel(t, t.TempDir())
	applyEffect(m, m.readTab(m.getTab()))

	_, cmd := m.handleRepeatShell()
	if cmd == nil {
		t.Fatal("expected a message command")
	}
	if got := lastMessage(t, m); got != "no shell history" {
		t.Fatalf("message %q", got)
	}
}

func TestRepeatShellRerunsTheLastCommand(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "a"))
	m := testModel(t, dir)
	applyEffect(m, m.readTab(m.getTab()))

	// Two runs, so the newest must win rather than the first.
	if err := saveShellHistory(nil, "echo first"); err != nil {
		t.Fatal(err)
	}
	history, err := loadShellHistory()
	if err != nil {
		t.Fatal(err)
	}
	if err := saveShellHistory(history, "echo second"); err != nil {
		t.Fatal(err)
	}

	// The key is handled in Normal mode; the returned command is deliberately
	// not applied, so no process is spawned by this test.
	keyEvent(m, ";")
	if got := lastMessage(t, m); got != "repeating: echo second" {
		t.Fatalf("message %q", got)
	}
	if m.mode != normalMode {
		t.Fatalf("mode changed to %v", m.mode)
	}
}

func TestRepeatShellExecutesInTheCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	m := testModel(t, dir)
	applyEffect(m, m.readTab(m.getTab()))

	const marker = "made-by-repeat.txt"
	if err := saveShellHistory(nil, "echo hi > "+marker); err != nil {
		t.Fatal(err)
	}

	_, cmd := m.handleRepeatShell()
	var proc *event.ProcessMsg
	for _, msg := range collectMsgs(cmd) {
		if p, ok := msg.(event.ProcessMsg); ok {
			proc = &p
		}
	}
	if proc == nil {
		t.Fatal("no process was scheduled")
	}
	// ExecProcess only wraps the command; the runtime runs it after suspending
	// the terminal, so stand in for the runtime here.
	if err := proc.Command.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, marker))
	if err != nil {
		t.Fatalf("command did not run in the tab's directory: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("command produced no output")
	}
}

func TestRepeatShellSeesCommandsRunSinceShellModeOpened(t *testing.T) {
	dir := t.TempDir()
	m := testModel(t, dir)
	applyEffect(m, m.readTab(m.getTab()))

	// The model's copy of the history is only refreshed when Shell mode opens,
	// so a command saved afterwards is invisible to it. Repeat must still find
	// the newest one.
	if err := saveShellHistory(nil, "echo stale"); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadShellHistory()
	if err != nil {
		t.Fatal(err)
	}
	m.shellHistory = loaded
	if err := saveShellHistory(loaded, "echo newest"); err != nil {
		t.Fatal(err)
	}

	keyEvent(m, ";")
	if got := lastMessage(t, m); got != "repeating: echo newest" {
		t.Fatalf("repeat used the stale in-memory history: %q", got)
	}
}
