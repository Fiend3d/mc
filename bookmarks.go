package main

import "slices"

type bookmarks struct {
	dirs     []string
	dirsCopy []string // it's for comparing
	cursor   int
	start    int
}

func (b *bookmarks) changed() bool {
	return !slices.Equal(b.dirs, b.dirsCopy)
}

func (b *bookmarks) moveCursor(move int, height int) {
	b.cursor += move
	b.cursor = min(b.cursor, len(b.dirs)-1)
	b.cursor = max(b.cursor, 0)
	b.updateStart(height)
}

func newBookmarks(dirs []string) *bookmarks {
	bookmarks := &bookmarks{
		dirs:     dirs,
		dirsCopy: make([]string, len(dirs)),
	}
	copy(bookmarks.dirsCopy, dirs)
	return bookmarks
}

func (b *bookmarks) updateStart(height int) {
	if b.cursor < b.start {
		b.start = b.cursor
		return
	}
	actualHeight := height - 3
	if b.cursor > b.start+actualHeight {
		b.start = b.cursor - actualHeight
	}
}
