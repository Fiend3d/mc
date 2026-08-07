package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// touch creates an empty file and fails the test if it can't.
func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
}

// An unedited name must stay put. Its path already exists - as itself - so a
// naive UniquePath call renames a file the user never touched.
func TestBuildRenamePairsKeepsUntouchedNames(t *testing.T) {
	dir := t.TempDir()

	names := []string{"a.txt", "b.txt", "c.txt"}
	paths := make([]string, len(names))
	for i, name := range names {
		paths[i] = filepath.Join(dir, name)
		touch(t, paths[i])
	}

	// only the middle one is edited
	pairs := buildRenamePairs(paths, []string{"a.txt", "renamed.txt", "c.txt"})

	if len(pairs) != 3 {
		t.Fatalf("expected 3 pairs, got %d", len(pairs))
	}
	if pairs[0].dst != paths[0] {
		t.Errorf("untouched a.txt became %q", filepath.Base(pairs[0].dst))
	}
	if pairs[2].dst != paths[2] {
		t.Errorf("untouched c.txt became %q", filepath.Base(pairs[2].dst))
	}
	if got := filepath.Base(pairs[1].dst); got != "renamed.txt" {
		t.Errorf("expected renamed.txt, got %q", got)
	}
}

// Windows paths are case-insensitive, so "foo.txt" -> "Foo.txt" must be a real
// rename rather than a collision that produces "Foo1.txt".
func TestBuildRenamePairsCaseOnly(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "foo.txt")
	touch(t, src)

	pairs := buildRenamePairs([]string{src}, []string{"Foo.txt"})

	if got := filepath.Base(pairs[0].dst); got != "Foo.txt" {
		t.Fatalf("expected Foo.txt, got %q", got)
	}
}

// A genuine collision with a file that isn't part of the rename still has to
// be renumbered rather than silently overwriting.
func TestBuildRenamePairsCollision(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	touch(t, src)
	touch(t, filepath.Join(dir, "taken.txt"))

	pairs := buildRenamePairs([]string{src}, []string{"taken.txt"})

	if got := filepath.Base(pairs[0].dst); got != "taken1.txt" {
		t.Fatalf("expected taken1.txt, got %q", got)
	}
}

// Two files renamed to the same new name must not collapse onto each other.
func TestBuildRenamePairsDuplicateNewNames(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	touch(t, a)
	touch(t, b)

	pairs := buildRenamePairs([]string{a, b}, []string{"same.txt", "same.txt"})

	if pairs[0].dst == pairs[1].dst {
		t.Fatalf("both files map to %q", pairs[0].dst)
	}
	if got := filepath.Base(pairs[0].dst); got != "same.txt" {
		t.Errorf("expected same.txt, got %q", got)
	}
	if got := filepath.Base(pairs[1].dst); got != "same1.txt" {
		t.Errorf("expected same1.txt, got %q", got)
	}
}

// newCreateCommand used to panic on an empty name (runes[-1]).
func TestNewCreateCommandEmptyName(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"", "file.txt", "sub\\", "sub/"} {
		cmd := newCreateCommand(name, dir)
		if cmd == nil {
			t.Fatalf("nil command for %q", name)
		}
		wantDir := strings.HasSuffix(name, "\\") || strings.HasSuffix(name, "/")
		if cmd.isDir != wantDir {
			t.Errorf("%q: isDir = %v, want %v", name, cmd.isDir, wantDir)
		}
	}
}

// A delete must never reach the undo history - its undo() always fails, which
// would wedge every command underneath it.
func TestDeleteCommandIsNotUndoable(t *testing.T) {
	if (&deleteCommand{}).undoable() {
		t.Error("deleteCommand claims to be undoable")
	}
	if !(&fileActionCommand{}).undoable() {
		t.Error("fileActionCommand should be undoable")
	}
	if !(&createCommand{}).undoable() {
		t.Error("createCommand should be undoable")
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty file", "", 0},
		{"trailing newline", "a\nb\n", 2},
		{"crlf", "a\r\nb\r\n", 2},
		{"blank lines", "a\n\n\nb", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.input)
			if len(got) != tt.want {
				t.Fatalf("splitLines(%q) = %v, want %d entries", tt.input, got, tt.want)
			}
			for _, line := range got {
				if strings.ContainsAny(line, "\r\n") {
					t.Errorf("entry %q still has line endings", line)
				}
			}
		})
	}
}
