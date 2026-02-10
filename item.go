package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dustin/go-humanize"
)

type item struct {
	entry    os.DirEntry
	fullPath string
	selected bool

	isDir     bool
	isSymlink bool
	name      string
	symlink   string
	modTime   string
	size      string
	mode      string
}

func newItem(entry os.DirEntry, dir string) (*item, error) {
	info, err := entry.Info()
	if err != nil {
		return nil, err
	}

	item := &item{entry: entry, selected: false}

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

	return item, nil
}
