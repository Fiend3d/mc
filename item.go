package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

type item interface {
	getName() string
	getFullPath() string
	isDirectory() bool
	getSize() uint64
	getModTime() time.Time
	isSelected() bool
	setSelected(bool)
	getAction() itemAction
	getExtra() string
}

type itemAction int

const (
	itemActionNone itemAction = iota
	itemActionCopy
	itemActionCut
)

type filepathItem struct {
	name     string
	fullPath string
	selected bool
	action   itemAction

	size uint64

	isDir      bool
	isSymlink  bool
	symlink    string
	modTimeStr string
	modTime    time.Time
	sizeStr    string
	mode       string

	// git is filled in after the listing, once git has answered.
	git gitState
}

func newFilepathItem(clipboardFiles []string, op OpType, entry os.DirEntry, dir string) (*filepathItem, error) {
	info, err := entry.Info()
	if err != nil {
		return nil, err
	}

	item := &filepathItem{selected: false}

	item.name = entry.Name()
	item.fullPath = filepath.Join(dir, item.name)
	item.isDir = info.IsDir()
	item.isSymlink = info.Mode()&os.ModeSymlink != 0

	if item.isSymlink {
		target, err := filepath.EvalSymlinks(filepath.Join(dir, item.name))
		if err != nil {
			return nil, err
		}
		stat, err := os.Stat(target)
		if err != nil {
			return nil, err
		}
		item.isDir = stat.IsDir()
		item.symlink = target
	}

	item.sizeStr = ""
	if !item.isDir {
		item.size = uint64(info.Size())
		item.sizeStr = strings.Replace(
			humanize.Bytes(item.size),
			" ",
			"",
			1,
		)
	}

	item.modTime = info.ModTime()
	item.modTimeStr = item.modTime.Format("02.01.2006 15:04")
	item.mode = info.Mode().String()

	if clipboardFiles != nil {
		if slices.Contains(clipboardFiles, item.fullPath) {
			switch op {
			case OpCopy:
				item.action = itemActionCopy
			case OpCut:
				item.action = itemActionCut
			}
		}
	}

	return item, nil
}

func (i *filepathItem) getName() string {
	return i.name
}

func (i *filepathItem) getFullPath() string {
	return i.fullPath
}

func (i *filepathItem) isDirectory() bool {
	return i.isDir
}

func (i *filepathItem) getSize() uint64 {
	return i.size
}

func (i *filepathItem) getModTime() time.Time {
	return i.modTime
}

func (i *filepathItem) isSelected() bool {
	return i.selected
}

func (i *filepathItem) setSelected(selected bool) {
	i.selected = selected
}

func (i *filepathItem) getAction() itemAction {
	return i.action
}

func (i *filepathItem) getExtra() string {
	return i.mode
}

type sharedItem struct {
	name     string
	fullPath string
	selected bool
	action   itemAction
}

func newSharedItem(clipboardFiles []string, op OpType, name string, fullPath string) *sharedItem {
	action := itemActionNone
	if clipboardFiles != nil {
		if slices.Contains(clipboardFiles, fullPath) {
			switch op {
			case OpCopy:
				action = itemActionCopy
			case OpCut:
				action = itemActionCut
			}
		}
	}
	return &sharedItem{name: name, fullPath: fullPath, action: action}
}

func (i *sharedItem) getName() string {
	return i.name
}

func (i *sharedItem) getFullPath() string {
	return i.fullPath
}

func (i *sharedItem) isDirectory() bool {
	return true
}

func (i *sharedItem) getSize() uint64 {
	return 0
}

func (i *sharedItem) getModTime() time.Time {
	var result time.Time
	return result
}

func (i *sharedItem) isSelected() bool {
	return i.selected
}

func (i *sharedItem) setSelected(selected bool) {
	i.selected = selected
}

func (i *sharedItem) getAction() itemAction {
	return i.action
}

func (i *sharedItem) getExtra() string {
	return ""
}

type driveItem struct {
	label     string
	selected  bool
	letter    string
	driveType string
	total     uint64
	free      uint64
	available uint64
}

func newDriveItem(d drive) *driveItem {
	return &driveItem{
		label:     d.name,
		letter:    d.letter,
		driveType: d.driveType,
		total:     d.total,
		free:      d.free,
		available: d.available,
	}
}

func (i *driveItem) getName() string {
	return i.letter
}

func (i *driveItem) getFullPath() string {
	return i.letter + "\\"
}

func (i *driveItem) isDirectory() bool {
	return true
}

func (i *driveItem) getSize() uint64 {
	return i.total
}

func (i *driveItem) getModTime() time.Time {
	var result time.Time
	return result
}

func (i *driveItem) isSelected() bool {
	return i.selected
}

func (i *driveItem) setSelected(selected bool) {
	i.selected = selected
}

func (i *driveItem) getAction() itemAction {
	return itemActionNone
}

func (i *driveItem) getExtra() string {
	if i.total == 0 {
		return ""
	}
	percent := (float64(i.free) / float64(i.total)) * 100
	return fmt.Sprintf("%.2f%% left", percent)
}
