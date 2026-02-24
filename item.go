package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/dustin/go-humanize"
)

type item interface {
	getName() string
	isDirectory() bool
	getFullPath() string
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

type filesystemItem struct {
	fullPath string
	selected bool
	action   itemAction

	isDir     bool
	isSymlink bool
	name      string
	symlink   string
	modTime   string
	size      string
	mode      string
}

func newFilesystemItem(entry os.DirEntry, dir string) (*filesystemItem, error) {
	info, err := entry.Info()
	if err != nil {
		return nil, err
	}

	item := &filesystemItem{selected: false}

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

	item.size = ""
	if !item.isDir {
		item.size = strings.Replace(
			humanize.Bytes(uint64(info.Size())),
			" ",
			"",
			1,
		)
	}

	item.modTime = info.ModTime().Format("02.01.2006 15:04")

	item.mode = info.Mode().String()

	paths, op, err := getClipboardFiles()
	if err == nil {
		if slices.Contains(paths, item.fullPath) {
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

func (i *filesystemItem) getName() string {
	return i.name
}

func (i *filesystemItem) isDirectory() bool {
	return i.isDir
}

func (i *filesystemItem) getFullPath() string {
	return i.fullPath
}

func (i *filesystemItem) isSelected() bool {
	return i.selected
}

func (i *filesystemItem) setSelected(selected bool) {
	i.selected = selected
}

func (i *filesystemItem) getAction() itemAction {
	return i.action
}

func (i *filesystemItem) render(s *strings.Builder, style *lipgloss.Style, t *theme, width int) {
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

	name = ansi.Truncate(name, nameWidth, "…")

	s.WriteString(name)
	nameLen := lipgloss.Width(name)
	if nameLen < nameWidth {
		s.WriteString(style.Width(nameWidth - nameLen).Render(" "))
	}

	// time column
	timeStyle := style.Foreground(t.grayColor)
	s.WriteString(timeStyle.Width(timeWidth).Render(i.modTime))

	s.WriteString(style.Render(" "))

	// size column
	s.WriteString(style.Render(
		lipgloss.PlaceHorizontal(sizeWidth, lipgloss.Center, i.size)))
	s.WriteString(style.Render(i.size))
}

func (i *filesystemItem) getExtra() string {
	return i.mode
}
