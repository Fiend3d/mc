package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
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
	render(s *strings.Builder, style *lipgloss.Style, t *theme, width int)
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

func (i *filepathItem) render(s *strings.Builder, style *lipgloss.Style, t *theme, width int) {
	const (
		sizeWidth = 8
		timeWidth = 16
		colGap    = 1
	)

	// name block
	var nameBlock strings.Builder

	nameWidth := max(
		width-sizeWidth-timeWidth-colGap*2+1, 1)

	if i.isDir {
		nameBlock.WriteString(
			style.Foreground(t.accentColor4).Render(i.name),
		)
		nameBlock.WriteString(style.Bold(true).Render("/"))
	} else {
		if strings.HasSuffix(strings.ToLower(i.name), ".exe") {
			nameBlock.WriteString(
				style.Foreground(t.greenColor).Render(i.name),
			)
		} else {
			nameBlock.WriteString(
				style.Foreground(t.whiteColor).Render(i.name),
			)
		}
	}

	if i.isSymlink {
		nameBlock.WriteString(
			style.Foreground(t.accentColor2).Render(" -> "))
		nameBlock.WriteString(
			style.Foreground(t.accentColor3).Render(i.symlink))
	}

	name := nameBlock.String()

	name = truncate(name, nameWidth)

	s.WriteString(name)
	nameLen := lipgloss.Width(name)
	if nameLen < nameWidth {
		s.WriteString(style.Width(nameWidth - nameLen).Render(" "))
	}

	// time column
	timeStyle := style.Foreground(t.grayColor)
	s.WriteString(timeStyle.Width(timeWidth).Render(i.modTimeStr))

	s.WriteString(style.Render(" "))

	// size column
	s.WriteString(style.Render(
		lipgloss.PlaceHorizontal(sizeWidth, lipgloss.Center, i.sizeStr)))
	s.WriteString(style.Render(i.sizeStr))
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

func (i *sharedItem) render(s *strings.Builder, style *lipgloss.Style, t *theme, width int) {
	info := " [shared] "
	infoSize := len(info)

	var nameBlock strings.Builder
	nameWidth := max(width-infoSize, 1)

	nameBlock.WriteString(
		style.Foreground(t.accentColor4).Render(i.name),
	)
	nameBlock.WriteString(style.Bold(true).Render("/"))

	name := nameBlock.String()

	name = truncate(name, nameWidth)

	s.WriteString(name)
	nameLen := lipgloss.Width(name)
	if nameLen < nameWidth {
		s.WriteString(style.Width(nameWidth - nameLen).Render(" "))
	}

	s.WriteString(style.Foreground(t.grayColor).Render(info))
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

func (i *driveItem) render(s *strings.Builder, style *lipgloss.Style, t *theme, width int) {
	var infoBlock strings.Builder
	sizeWidth := 8
	infoBlock.WriteString(style.Foreground(t.grayColor).Render(fmt.Sprintf("%s ", i.driveType)))
	infoBlock.WriteString(style.Align(lipgloss.Right).Width(sizeWidth).Render(humanize.Bytes(i.free)))
	infoBlock.WriteString(style.Render(" free of "))
	infoBlock.WriteString(style.
		Align(lipgloss.Left).
		Foreground(t.accentColor5).
		Bold(true).
		Width(sizeWidth).
		Render(humanize.Bytes(i.total)))
	infoBlock.WriteString(style.Render(" "))

	info := infoBlock.String()
	infoSize := lipgloss.Width(info)

	var nameBlock strings.Builder
	nameWidth := max(width-infoSize, 1)

	nameBlock.WriteString(
		style.Foreground(t.accentColor4).Render(i.letter),
	)
	nameBlock.WriteString(style.Bold(true).Render("/"))
	nameBlock.WriteString(style.Foreground(t.accentColor2).Render(fmt.Sprintf(" %s", i.label)))
	name := nameBlock.String()

	name = truncate(name, nameWidth)

	s.WriteString(name)
	nameLen := lipgloss.Width(name)
	if nameLen < nameWidth {
		s.WriteString(style.Width(nameWidth - nameLen).Render(" "))
	}

	s.WriteString(style.Foreground(t.grayColor).Render(info))
}

func (i *driveItem) getExtra() string {
	percent := (float64(i.free) / float64(i.total)) * 100
	return fmt.Sprintf("%.2f%% left", percent)
}
