package shutil

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Progress reports actual bytes written. Total is known after enumeration.
type Progress struct {
	Path              string
	Bytes, Total      int64
	Files, TotalFiles int
	// Steps and TotalSteps carry coarse completion for work whose byte total is
	// unknowable up front, such as a size walk counting top-level entries.
	Steps, TotalSteps int
	Scanning          bool
}
type ProgressFunc func(Progress)
type Record struct {
	Source, Destination string
	Move, Directory     bool
	Snapshot            map[string]Stamp
	RemovedSourceDir    bool
}
type Stamp struct {
	Size     int64
	Modified int64
	Mode     os.FileMode
}

func CheckPath(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return err
	}
	if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("unsupported reparse point: %s", path)
	}
	return nil
}
func Within(parent, child string) bool {
	a, e1 := filepath.Abs(parent)
	b, e2 := filepath.Abs(child)
	if e1 != nil || e2 != nil {
		return false
	}
	a = strings.ToLower(filepath.Clean(a))
	b = strings.ToLower(filepath.Clean(b))
	rel, err := filepath.Rel(a, b)
	return err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}
func CheckDestination(path string) error {
	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		if _, err := os.Lstat(p); err == nil {
			if err = CheckPath(p); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}
func Snapshot(path string) (map[string]Stamp, error) {
	result := map[string]Stamp{}
	err := filepath.Walk(path, func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if e = CheckPath(p); e != nil {
			return e
		}
		r, e := filepath.Rel(path, p)
		if e != nil {
			return e
		}
		result[r] = Stamp{i.Size(), i.ModTime().UnixNano(), i.Mode()}
		return nil
	})
	return result, err
}
func Unchanged(path string, want map[string]Stamp) error {
	got, err := Snapshot(path)
	if err != nil {
		return err
	}
	if len(got) != len(want) {
		return fmt.Errorf("path changed since operation: %s", path)
	}
	for p, v := range want {
		g, ok := got[p]
		if !ok || g != v {
			return fmt.Errorf("path changed since operation: %s", filepath.Join(path, p))
		}
	}
	return nil
}

// Transfer copies or moves a single root and returns a journal even on failure.
// Existing files are replaced only after their temporary copy has been synced.
func Transfer(ctx context.Context, src, dst string, move, overwrite bool, report ProgressFunc) (journal []Record, err error) {
	return transfer(ctx, src, dst, move, overwrite, report, true)
}

func transfer(ctx context.Context, src, dst string, move, overwrite bool, report ProgressFunc, tryRename bool) (journal []Record, err error) {
	if err = CheckDestination(src); err != nil {
		return nil, err
	}
	if strings.EqualFold(src, dst) && src != dst && move {
		if err = CheckPath(src); err != nil {
			return nil, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if err = os.Rename(src, dst); err != nil {
			return nil, err
		}
		snap, e := Snapshot(dst)
		return []Record{{Source: src, Destination: dst, Move: true, Snapshot: snap}}, e
	}
	if Within(src, dst) {
		return nil, fmt.Errorf("destination is the source or inside it: %s", dst)
	}
	if err = CheckDestination(dst); err != nil {
		return nil, err
	}
	type entry struct {
		src, dst string
		info     os.FileInfo
	}
	var entries []entry
	progress := Progress{Path: src, Scanning: true}
	if report != nil {
		report(progress)
	}
	err = filepath.Walk(src, func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if e = ctx.Err(); e != nil {
			return e
		}
		if e = CheckPath(p); e != nil {
			return e
		}
		if !i.IsDir() && !i.Mode().IsRegular() {
			return fmt.Errorf("unsupported file: %s", p)
		}
		rel, e := filepath.Rel(src, p)
		if e != nil {
			return e
		}
		entries = append(entries, entry{p, filepath.Join(dst, rel), i})
		if !i.IsDir() {
			progress.Total += i.Size()
			progress.TotalFiles++
		}
		return nil
	})
	if err != nil {
		return journal, err
	}
	progress.Scanning = false
	if report != nil {
		report(progress)
	}
	if err = ctx.Err(); err != nil {
		return journal, err
	}
	if move && tryRename && !PathExists(dst) {
		if err = os.Rename(src, dst); err == nil {
			snap, e := Snapshot(dst)
			journal = append(journal, Record{Source: src, Destination: dst, Move: true, Directory: len(entries) > 1 || entries[0].info.IsDir(), Snapshot: snap})
			progress.Bytes = progress.Total
			progress.Files = progress.TotalFiles
			if report != nil {
				report(progress)
			}
			return journal, e
		}
	}
	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return journal, err
		}
		progress.Path = entry.src
		if err = CheckDestination(entry.dst); err != nil {
			return journal, err
		}
		if entry.info.IsDir() {
			if !PathExists(entry.dst) {
				if err = os.MkdirAll(entry.dst, entry.info.Mode()); err != nil {
					return journal, err
				}
				journal = append(journal, Record{Source: entry.src, Destination: entry.dst, Directory: true})
			}
			continue
		}
		if err = copyAtomic(ctx, entry.src, entry.dst, entry.info, overwrite, func(n int64) {
			progress.Bytes += n
			if report != nil {
				report(progress)
			}
		}); err != nil {
			return journal, err
		}
		record := Record{Source: entry.src, Destination: entry.dst}
		if move {
			if err = os.Remove(entry.src); err == nil {
				record.Move = true
			}
		}
		record.Snapshot, _ = Snapshot(entry.dst)
		journal = append(journal, record)
		if err != nil {
			return journal, err
		}
		progress.Files++
		if report != nil {
			report(progress)
		}
	}
	if move {
		for i := len(entries) - 1; i >= 0; i-- {
			if entries[i].info.IsDir() {
				if err = ctx.Err(); err != nil {
					return journal, err
				}
				if err = os.Remove(entries[i].src); err != nil {
					return journal, err
				}
				journal = append(journal, Record{Source: entries[i].src, RemovedSourceDir: true})
			}
		}
	}
	return journal, nil
}
func copyAtomic(ctx context.Context, src, dst string, info os.FileInfo, overwrite bool, progress func(int64)) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err = os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}
	out, err := os.CreateTemp(filepath.Dir(dst), ".mc-copy-*")
	if err != nil {
		return err
	}
	tmp := out.Name()
	defer os.Remove(tmp)
	defer out.Close()
	buf := make([]byte, 256*1024)
	var written int64
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		n, re := in.Read(buf)
		if n > 0 {
			nw, we := out.Write(buf[:n])
			written += int64(nw)
			if progress != nil {
				progress(int64(nw))
			}
			if we != nil {
				return we
			}
			if nw != n {
				return io.ErrShortWrite
			}
		}
		if re == io.EOF {
			break
		}
		if re != nil {
			return re
		}
	}
	current, statErr := in.Stat()
	if statErr != nil {
		return statErr
	}
	if written != info.Size() || current.Size() != info.Size() || !current.ModTime().Equal(info.ModTime()) {
		return fmt.Errorf("source changed during copy: %s", src)
	}
	if err = out.Chmod(info.Mode()); err != nil {
		return err
	}
	if err = out.Sync(); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	if err = os.Chtimes(tmp, time.Now(), info.ModTime()); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	a, e := windows.UTF16PtrFromString(tmp)
	if e != nil {
		return e
	}
	b, e := windows.UTF16PtrFromString(dst)
	if e != nil {
		return e
	}
	flags := uint32(windows.MOVEFILE_WRITE_THROUGH)
	if overwrite {
		flags |= windows.MOVEFILE_REPLACE_EXISTING
	}
	return windows.MoveFileEx(a, b, flags)
}

// Undo checks the complete journal before changing anything, and then reverses
// completed entries. Newly created directories are removed only when empty.
func Undo(ctx context.Context, records []Record) error {
	var undone []Record
	return UndoJournal(ctx, &records, &undone, nil)
}
func UndoJournal(ctx context.Context, records, undone *[]Record, report ProgressFunc) error {
	for _, r := range *records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if (!r.Directory || r.Move) && !r.RemovedSourceDir && len(r.Snapshot) == 0 {
			return fmt.Errorf("cannot verify undo path: %s", r.Destination)
		}
		if len(r.Snapshot) > 0 {
			if err := Unchanged(r.Destination, r.Snapshot); err != nil {
				return err
			}
		}
		if r.Move && PathExists(r.Source) && !strings.EqualFold(r.Source, r.Destination) {
			return fmt.Errorf("undo destination already exists: %s", r.Source)
		}
		if r.RemovedSourceDir && PathExists(r.Source) {
			info, err := os.Stat(r.Source)
			if err != nil || !info.IsDir() {
				return fmt.Errorf("undo directory conflict: %s", r.Source)
			}
		}
	}
	total := len(*records)
	for len(*records) > 0 {
		i := len(*records) - 1
		r := (*records)[i]
		if err := ctx.Err(); err != nil {
			return err
		}
		if r.RemovedSourceDir {
			if err := os.MkdirAll(r.Source, 0755); err != nil {
				return err
			}
		} else if r.Move {
			if _, err := Transfer(ctx, r.Destination, r.Source, true, false, report); err != nil {
				return err
			}
		} else {
			if err := os.Remove(r.Destination); err != nil {
				return err
			}
		}
		*undone = append(*undone, r)
		*records = (*records)[:i]
		if report != nil {
			report(Progress{Path: r.Destination, Files: total - i, TotalFiles: total})
		}
	}
	return nil
}
