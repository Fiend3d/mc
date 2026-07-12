package shutil

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestIsDir(t *testing.T) {
	tmpDir := t.TempDir()
	if !IsDir(tmpDir) {
		t.Fatal("expected temp dir to be a directory")
	}
	if IsDir(filepath.Join(tmpDir, "nonexistent")) {
		t.Fatal("expected nonexistent to be false")
	}
	if IsDir("") {
		t.Fatal("expected empty string to be false")
	}

	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if IsDir(tmpFile) {
		t.Fatal("expected file to not be a directory")
	}
}

func TestIsFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if !IsFile(tmpFile) {
		t.Fatal("expected test file to be a file")
	}
	if IsFile(filepath.Join(tmpDir, "nonexistent")) {
		t.Fatal("expected nonexistent to be false")
	}
	if IsFile("") {
		t.Fatal("expected empty string to be false")
	}
	if IsFile(tmpDir) {
		t.Fatal("expected directory to not be a file")
	}
}

func TestDirExists(t *testing.T) {
	tmpDir := t.TempDir()
	if !DirExists(tmpDir) {
		t.Fatal("expected temp dir to exist")
	}
	if DirExists(filepath.Join(tmpDir, "nonexistent")) {
		t.Fatal("expected nonexistent to be false")
	}
}

func TestPathExists(t *testing.T) {
	tmpDir := t.TempDir()
	if !PathExists(tmpDir) {
		t.Fatal("expected temp dir to exist")
	}
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if !PathExists(tmpFile) {
		t.Fatal("expected file to exist")
	}
	if PathExists(filepath.Join(tmpDir, "nonexistent")) {
		t.Fatal("expected nonexistent to be false")
	}
}

func TestIsEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	empty, err := IsEmpty(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if !empty {
		t.Fatal("expected empty dir to be empty")
	}

	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	empty, err = IsEmpty(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if empty {
		t.Fatal("expected non-empty dir to not be empty")
	}
}

func TestIsEmptyNonexistent(t *testing.T) {
	_, err := IsEmpty(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "src.txt")
	if err := os.WriteFile(srcFile, []byte("test content"), 0600); err != nil {
		t.Fatal(err)
	}

	srcInfo, err := os.Stat(srcFile)
	if err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(tmpDir, "dst.txt")
	if err := CopyFile(srcFile, dstFile); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "test content" {
		t.Fatalf("expected 'test content', got '%s'", string(content))
	}

	dstInfo, err := os.Stat(dstFile)
	if err != nil {
		t.Fatal(err)
	}
	if srcInfo.Mode() != dstInfo.Mode() {
		t.Fatalf("mode mismatch: %v vs %v", srcInfo.Mode(), dstInfo.Mode())
	}

	if err := CopyFile("notfound.txt", dstFile); err == nil {
		t.Fatal("expected error for nonexistent source")
	}

	if err := CopyFile(srcFile, ""); err == nil {
		t.Fatal("expected error for invalid dest")
	}
}

func TestCopyFileToSubDir(t *testing.T) {
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "src.txt")
	if err := os.WriteFile(srcFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(tmpDir, "sub", "nested", "dst.txt")
	if err := CopyFile(srcFile, dstFile); err != nil {
		t.Fatal(err)
	}

	if !IsFile(dstFile) {
		t.Fatal("expected destination file to exist")
	}
}

func TestCopyDir(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "copied")

	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyDir(srcDir, dstDir); err != nil {
		t.Fatal(err)
	}

	if !IsDir(dstDir) {
		t.Fatal("expected destination dir to exist")
	}
	if !IsDir(filepath.Join(dstDir, "subdir")) {
		t.Fatal("expected subdir to exist")
	}
	if !IsFile(filepath.Join(dstDir, "file1.txt")) {
		t.Fatal("expected file1.txt to exist")
	}
	if !IsFile(filepath.Join(dstDir, "subdir", "file2.txt")) {
		t.Fatal("expected file2.txt to exist")
	}

	content, err := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "content1" {
		t.Fatalf("expected 'content1', got '%s'", string(content))
	}
}

func TestCopyEmptyDir(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "copied")

	if err := os.MkdirAll(filepath.Join(srcDir, "emptysub"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := CopyDir(srcDir, dstDir); err != nil {
		t.Fatal(err)
	}

	if !IsDir(filepath.Join(dstDir, "emptysub")) {
		t.Fatal("expected empty subdirectory to be copied")
	}

	empty, err := IsEmpty(filepath.Join(dstDir, "emptysub"))
	if err != nil {
		t.Fatal(err)
	}
	if !empty {
		t.Fatal("expected copied empty subdir to remain empty")
	}
}

func TestCopyDirNonexistent(t *testing.T) {
	if err := CopyDir(filepath.Join(t.TempDir(), "nonexistent"), t.TempDir()); err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestListFiles(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := ListFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if !strings.HasSuffix(files[0], "a.txt") || !strings.HasSuffix(files[1], "b.txt") {
		t.Fatalf("unexpected file list: %v", files)
	}

	if _, err := ListFiles(filepath.Join(dir, "nonexistent")); err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestListFilesEmptyDir(t *testing.T) {
	dir := t.TempDir()

	// create an empty subdir
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0755); err != nil {
		t.Fatal(err)
	}

	files, err := ListFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestMoveFile(t *testing.T) {
	t.Run("same device", func(t *testing.T) {
		tmpDir := t.TempDir()
		srcFile := filepath.Join(tmpDir, "src.txt")
		dstFile := filepath.Join(tmpDir, "dst.txt")

		if err := os.WriteFile(srcFile, []byte("test content"), 0600); err != nil {
			t.Fatal(err)
		}

		if err := MoveFile(srcFile, dstFile); err != nil {
			t.Fatal(err)
		}

		if PathExists(srcFile) {
			t.Fatal("source file should not exist after move")
		}

		content, err := os.ReadFile(dstFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "test content" {
			t.Fatalf("expected 'test content', got '%s'", string(content))
		}
	})

	t.Run("move to new subdir", func(t *testing.T) {
		tmpDir := t.TempDir()
		srcFile := filepath.Join(tmpDir, "src.txt")
		dstFile := filepath.Join(tmpDir, "subdir", "nested", "dst.txt")

		if err := os.WriteFile(srcFile, []byte("test content"), 0600); err != nil {
			t.Fatal(err)
		}

		if err := MoveFile(srcFile, dstFile); err != nil {
			t.Fatal(err)
		}

		if PathExists(srcFile) {
			t.Fatal("source file should not exist after move")
		}

		content, err := os.ReadFile(dstFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "test content" {
			t.Fatalf("expected 'test content', got '%s'", string(content))
		}
	})

	t.Run("errors", func(t *testing.T) {
		tests := []struct {
			name    string
			src     string
			dst     string
			wantErr string
		}{
			{"source not found", "notfound.txt", "dst.txt", "source file not found"},
			{"empty source", "", "dst.txt", "empty source path"},
			{"empty destination", "src.txt", "", "empty destination path"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if err := MoveFile(tt.src, tt.dst); err == nil {
					t.Fatal("expected error")
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			})
		}
	})
}

func TestTouchFile(t *testing.T) {
	t.Run("create new", func(t *testing.T) {
		tmpDir := t.TempDir()
		newFile := filepath.Join(tmpDir, "new.txt")

		if err := TouchFile(newFile); err != nil {
			t.Fatal(err)
		}

		info, err := os.Stat(newFile)
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() != 0 {
			t.Fatalf("expected empty file, got size %d", info.Size())
		}
		if time.Since(info.ModTime()) > time.Second {
			t.Fatal("expected recent mod time")
		}
	})

	t.Run("update existing", func(t *testing.T) {
		tmpDir := t.TempDir()
		existingFile := filepath.Join(tmpDir, "existing.txt")

		if err := os.WriteFile(existingFile, []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}

		origInfo, err := os.Stat(existingFile)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)

		if err := TouchFile(existingFile); err != nil {
			t.Fatal(err)
		}

		newInfo, err := os.Stat(existingFile)
		if err != nil {
			t.Fatal(err)
		}
		if newInfo.Size() != origInfo.Size() {
			t.Fatalf("content size changed: %d vs %d", origInfo.Size(), newInfo.Size())
		}
		if !newInfo.ModTime().After(origInfo.ModTime()) {
			t.Fatal("expected mod time to be updated")
		}
	})

	t.Run("create in subdir", func(t *testing.T) {
		tmpDir := t.TempDir()
		newFile := filepath.Join(tmpDir, "sub", "nested", "new.txt")

		if err := TouchFile(newFile); err != nil {
			t.Fatal(err)
		}

		if !IsFile(newFile) {
			t.Fatal("expected file to exist")
		}
	})

	t.Run("errors", func(t *testing.T) {
		if err := TouchFile(""); err == nil {
			t.Fatal("expected error for empty path")
		}
	})
}

func TestCalcDirSize(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "sub", "b.txt"), []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}

	size, err := CalcDirSize(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if size != 10 {
		t.Fatalf("expected size 10, got %d", size)
	}
}

func TestCalcDirSizeEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	size, err := CalcDirSize(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if size != 0 {
		t.Fatalf("expected size 0, got %d", size)
	}
}

func TestUniquePath(t *testing.T) {
	tmpDir := t.TempDir()

	// no collision - returns original
	path := filepath.Join(tmpDir, "test.txt")
	result := UniquePath(nil, nil, path)
	if result != path {
		t.Fatalf("expected %s, got %s", path, result)
	}

	// with collision - should get numbered version
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	result = UniquePath(nil, nil, path)
	expected := filepath.Join(tmpDir, "test1.txt")
	if result != expected {
		t.Fatalf("expected %s, got %s", expected, result)
	}

	// create another, should get test2
	path2 := filepath.Join(tmpDir, "test1.txt")
	if err := os.WriteFile(path2, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	result = UniquePath(nil, nil, path)
	expected = filepath.Join(tmpDir, "test2.txt")
	if result != expected {
		t.Fatalf("expected %s, got %s", expected, result)
	}

	// already numbered, no collision
	path3 := filepath.Join(tmpDir, "test10.txt")
	result = UniquePath(nil, nil, path3)
	if result != path3 {
		t.Fatalf("expected %s, got %s", path3, result)
	}
}

func TestUniquePathWithReserved(t *testing.T) {
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "test.txt")
	reserved := []string{path}
	result := UniquePath(reserved, nil, path)
	expected := filepath.Join(tmpDir, "test1.txt")
	if result != expected {
		t.Fatalf("expected %s, got %s", expected, result)
	}
}

func TestUniquePathWithExclude(t *testing.T) {
	tmpDir := t.TempDir()

	basePath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(basePath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// create test1.txt on disk
	path1 := filepath.Join(tmpDir, "test1.txt")
	if err := os.WriteFile(path1, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// without exclude, skips test1.txt, returns test2.txt
	result := UniquePath(nil, nil, basePath)
	if result != filepath.Join(tmpDir, "test2.txt") {
		t.Fatalf("expected test2.txt, got %s", result)
	}

	// remove test1.txt from disk, but exclude it from the scan
	// (simulating a move where test1.txt no longer exists)
	if err := os.Remove(path1); err != nil {
		t.Fatal(err)
	}

	// with test1.txt excluded, it's not counted as existing
	// test2.txt doesn't exist, so it should be available
	result = UniquePath(nil, []string{path1}, basePath)
	if result != filepath.Join(tmpDir, "test1.txt") {
		t.Fatalf("expected test1.txt (excluded), got %s", result)
	}
}

func TestParseName(t *testing.T) {
	tests := []struct {
		input     string
		baseName  string
		number    int
		width     int
		hasNumber bool
	}{
		{"test", "test", 0, 1, false},
		{"test01", "test", 1, 2, true},
		{"test123", "test", 123, 3, true},
		{"abc001def", "abc001def", 0, 1, false},
		{"", "", 0, 1, false},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			base, num, width, hasNum := parseName(tt.input)
			if base != tt.baseName || num != tt.number || width != tt.width || hasNum != tt.hasNumber {
				t.Fatalf("parseName(%q) = (%q, %d, %d, %v), expected (%q, %d, %d, %v)",
					tt.input, base, num, width, hasNum, tt.baseName, tt.number, tt.width, tt.hasNumber)
			}
		})
	}
}
