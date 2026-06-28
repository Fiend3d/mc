package shutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

func exists(name string, dir bool) bool {
	info, err := os.Stat(name)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return false
	}
	if dir {
		return info.IsDir()
	}
	return !info.IsDir()
}

func IsDir(path string) bool { return exists(path, true) }

func IsFile(path string) bool { return exists(path, false) }

func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func IsEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func CopyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("can't stat %s: %w", src, err)
	}

	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("can't copy non-regular source file %s (%s)", src, srcInfo.Mode().String())
	}

	srcFh, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("can't open source file %s: %w", src, err)
	}
	defer srcFh.Close()

	if err = os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return fmt.Errorf("can't make destination directory %s: %w", filepath.Dir(dst), err)
	}

	dstFh, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("can't create destination file %s: %w", dst, err)
	}
	defer dstFh.Close()

	size, err := io.Copy(dstFh, srcFh)
	if err != nil {
		return fmt.Errorf("can't copy data: %w", err)
	}
	if size != srcInfo.Size() {
		return fmt.Errorf("incomplete copy, %d of %d", size, srcInfo.Size())
	}

	return dstFh.Sync()
}

func CopyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return CopyFile(path, destPath)
	})
}

func ListFiles(dir string) ([]string, error) {
	var list []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		list = append(list, path)
		return nil
	})
	sort.Slice(list, func(i, j int) bool {
		return list[i] < list[j]
	})
	return list, err
}

func MoveFile(src, dst string) error {
	if src == "" {
		return fmt.Errorf("empty source path")
	}
	if dst == "" {
		return fmt.Errorf("empty destination path")
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("source file not found: %s", src)
		}
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("source is not a regular file: %s", src)
	}

	if err = os.Rename(src, dst); err == nil {
		return nil
	}

	if err = os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	if err = os.Rename(src, dst); err == nil {
		return nil
	}

	if err = CopyFile(src, dst); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	dstInfo, err := os.Stat(dst)
	if err != nil {
		return fmt.Errorf("failed to stat destination file: %w", err)
	}
	if srcInfo.Size() != dstInfo.Size() {
		return fmt.Errorf("size mismatch after copy: source %d, destination %d", srcInfo.Size(), dstInfo.Size())
	}

	if err := os.Remove(src); err != nil {
		return fmt.Errorf("failed to remove source file: %w", err)
	}

	return nil
}

func TouchFile(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}

	_, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to stat file: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("failed to close file: %w", err)
		}
		return nil
	}

	now := time.Now()
	return os.Chtimes(path, now, now)
}

func CalcDirSize(path string) (uint64, error) {
	var size uint64

	err := filepath.WalkDir(path, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return nil
			}
			size += uint64(info.Size())
		}
		return nil
	})

	return size, err
}

func UniquePath(reserved []string, exclude []string, path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	baseName, number, width, hasNumber := parseName(name)

	if !hasNumber {
		if !PathExists(path) && !slices.Contains(reserved, path) {
			return path
		}
	}

	existing := findExistingNumbers(reserved, exclude, dir, baseName, ext)

	used := map[int]struct{}{}
	for _, n := range existing {
		used[n] = struct{}{}
	}

	next := number
	if !hasNumber {
		next = 1
	}

	for {
		if _, ok := used[next]; !ok {
			candidate := filepath.Join(dir,
				fmt.Sprintf("%s%0*d%s", baseName, width, next, ext))

			if !PathExists(candidate) && !slices.Contains(reserved, candidate) {
				return candidate
			}
		}
		next++
	}
}

func parseName(name string) (baseName string, number int, width int, hasNumber bool) {
	re := regexp.MustCompile(`^(.*?)(\d+)$`)
	m := re.FindStringSubmatch(name)

	if len(m) == 3 {
		n, _ := strconv.Atoi(m[2])
		return m[1], n, len(m[2]), true
	}

	return name, 0, 1, false
}

func findExistingNumbers(reserved []string, exclude []string, dir, baseName, ext string) []int {
	var nums []int
	pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(baseName) + `(\d+)` + regexp.QuoteMeta(ext) + `$`)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nums
	}

	for _, entry := range entries {
		name := entry.Name()
		entryPath := filepath.Join(dir, name)
		if slices.Contains(exclude, entryPath) {
			continue
		}
		matches := pattern.FindStringSubmatch(name)
		if len(matches) == 2 {
			num, _ := strconv.Atoi(matches[1])
			if num > 0 {
				nums = append(nums, num)
			}
		}
	}

	for _, rp := range reserved {
		name := filepath.Base(rp)
		matches := pattern.FindStringSubmatch(name)
		if len(matches) == 2 {
			num, _ := strconv.Atoi(matches[1])
			if num > 0 {
				nums = append(nums, num)
			}
		}
	}

	sort.Ints(nums)
	return nums
}
