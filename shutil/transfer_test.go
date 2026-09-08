package shutil

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeTransferFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
func TestTransferCancellationPreservesExistingDestination(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "source"), filepath.Join(root, "target")
	writeTransferFile(t, src, bytes.Repeat([]byte("x"), 1024*1024))
	writeTransferFile(t, dst, []byte("original"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	journal, err := Transfer(ctx, src, dst, false, true, func(p Progress) {
		if p.Bytes > 0 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if len(journal) != 0 {
		t.Fatal("unfinished file journaled")
	}
	data, _ := os.ReadFile(dst)
	if string(data) != "original" {
		t.Fatal("existing destination changed")
	}
	temps, _ := filepath.Glob(filepath.Join(root, ".mc-copy-*"))
	if len(temps) > 0 {
		t.Fatalf("temporary files remain: %v", temps)
	}
}
func TestTransferNestedCopyAndUndo(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	writeTransferFile(t, filepath.Join(src, "sub", "data"), []byte("hello"))
	writeTransferFile(t, filepath.Join(src, "empty"), nil)
	if err := os.MkdirAll(filepath.Join(src, "emptydir"), 0755); err != nil {
		t.Fatal(err)
	}
	var last Progress
	journal, err := Transfer(context.Background(), src, dst, false, false, func(p Progress) { last = p })
	if err != nil {
		t.Fatal(err)
	}
	if last.Bytes != 5 || last.Total != 5 || last.Files != 2 {
		t.Fatalf("wrong progress: %+v", last)
	}
	if err = Undo(context.Background(), journal); err != nil {
		t.Fatal(err)
	}
	if PathExists(dst) {
		t.Fatal("destination remains")
	}
	if !PathExists(src) {
		t.Fatal("source removed")
	}
}
func TestUndoRefusesChangedFiles(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	writeTransferFile(t, src, []byte("hello"))
	journal, err := Transfer(context.Background(), src, dst, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	writeTransferFile(t, dst, []byte("new content"))
	if err = Undo(context.Background(), journal); err == nil {
		t.Fatal("undo accepted changed file")
	}
	data, _ := os.ReadFile(dst)
	if string(data) != "new content" {
		t.Fatal("changed file lost")
	}
}
func TestMoveAndCaseRenameUndo(t *testing.T) {
	for _, caseOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "move", true: "case"}[caseOnly], func(t *testing.T) {
			root := t.TempDir()
			src, dst := filepath.Join(root, "source"), filepath.Join(root, "destination")
			if caseOnly {
				dst = filepath.Join(root, "Source")
			}
			writeTransferFile(t, src, []byte("hello"))
			journal, err := Transfer(context.Background(), src, dst, true, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err = Undo(context.Background(), journal); err != nil {
				t.Fatal(err)
			}
			if !PathExists(src) {
				t.Fatal("source not restored")
			}
		})
	}
}
func TestTransferRejectsDescendantAndCollision(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	writeTransferFile(t, filepath.Join(src, "file"), []byte("a"))
	if _, err := Transfer(context.Background(), src, filepath.Join(src, "child"), false, false, nil); err == nil {
		t.Fatal("descendant accepted")
	}
	dst := filepath.Join(root, "existing")
	writeTransferFile(t, dst, []byte("keep"))
	if _, err := Transfer(context.Background(), filepath.Join(src, "file"), dst, false, false, nil); err == nil {
		t.Fatal("collision accepted")
	}
	data, _ := os.ReadFile(dst)
	if string(data) != "keep" {
		t.Fatal("collision overwrote file")
	}
}
func TestPartialCopyCanBeUndone(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	writeTransferFile(t, filepath.Join(src, "a"), []byte("one"))
	writeTransferFile(t, filepath.Join(src, "b"), []byte("two"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	journal, err := Transfer(ctx, src, dst, false, false, func(p Progress) {
		if p.Files == 1 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if !PathExists(filepath.Join(dst, "a")) || PathExists(filepath.Join(dst, "b")) {
		t.Fatal("incorrect partial state")
	}
	if err = Undo(context.Background(), journal); err != nil {
		t.Fatal(err)
	}
	if PathExists(dst) {
		t.Fatal("partial destination remains")
	}
}

func TestDirectoryMoveFallbackRestoresEmptyDirectories(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "src"), filepath.Join(root, "dst")
	writeTransferFile(t, filepath.Join(src, "sub", "data"), []byte("hello"))
	if err := os.MkdirAll(filepath.Join(src, "empty"), 0755); err != nil {
		t.Fatal(err)
	}
	// Force the copy/delete path used when rename crosses volume boundaries.
	journal, err := transfer(context.Background(), src, dst, true, false, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if PathExists(src) {
		t.Fatal("move left source behind")
	}
	if err = Undo(context.Background(), journal); err != nil {
		t.Fatal(err)
	}
	if !PathExists(filepath.Join(src, "empty")) || !PathExists(filepath.Join(src, "sub", "data")) || PathExists(dst) {
		t.Fatal("fallback undo did not restore the original tree")
	}
}
