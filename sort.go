package main

import (
	"math/rand"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type sortMethod int

const (
	modifiedTimeSort sortMethod = iota
	alphabeticSort
	normalSort
	extensionSort
	sizeSort
	randomSort
)

func hasExtension(path string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(base)

	if ext == "" {
		return false
	}

	if ext == base {
		return false
	}

	return true
}

func (m *model) sort(method sortMethod, reverse bool) { m.getTab().sortItems(method, reverse) }
func (t *tab) sortItems(method sortMethod, reverse bool) {
	t.page.selectionRange = nil
	t.sorted, t.sortMethod, t.sortReverse = true, method, reverse
	items := t.page.getItems()
	if len(items) == 0 {
		return
	}
	settings := t.getPageSettings()
	settings.update(len(items))
	selectedItem := items[settings.cursor]

	// Reversing by negating the result would make less(i,j) and less(j,i) both
	// true for equal keys, which isn't a strict weak ordering - swap the
	// operands instead.
	pair := func(i, j int) (item, item) {
		if reverse {
			return items[j], items[i]
		}
		return items[i], items[j]
	}

	switch method {
	case alphabeticSort:
		sort.Slice(items, func(i, j int) bool {
			a, b := pair(i, j)
			return strings.ToLower(a.getName()) < strings.ToLower(b.getName())
		})
	case extensionSort:
		sort.Slice(items, func(i, j int) bool {
			a, b := pair(i, j)
			aIsDir := a.isDirectory()
			bIsDir := b.isDirectory()
			if aIsDir != bIsDir {
				return aIsDir // directories first
			}
			if aIsDir {
				return strings.ToLower(a.getName()) < strings.ToLower(b.getName())
			}
			aHasExt := hasExtension(a.getName())
			bHasExt := hasExtension(b.getName())
			if aHasExt != bHasExt {
				return aHasExt // files with an extension first
			}
			if aHasExt {
				aExt := strings.ToLower(filepath.Ext(a.getName()))
				bExt := strings.ToLower(filepath.Ext(b.getName()))
				if aExt != bExt {
					return aExt < bExt
				}
			}
			return strings.ToLower(a.getName()) < strings.ToLower(b.getName())
		})
	case modifiedTimeSort:
		sort.Slice(items, func(i, j int) bool {
			a, b := pair(i, j)
			return a.getModTime().After(b.getModTime())
		})
	case normalSort:
		sort.Slice(items, func(i, j int) bool {
			a, b := pair(i, j)
			aIsDir := a.isDirectory()
			bIsDir := b.isDirectory()
			if aIsDir != bIsDir {
				return aIsDir // directories first
			}
			return strings.ToLower(a.getName()) < strings.ToLower(b.getName())
		})
	case sizeSort:
		sort.Slice(items, func(i, j int) bool {
			a, b := items[i], items[j]
			aUnknown, bUnknown := hasUnknownDirectorySize(a), hasUnknownDirectorySize(b)
			if aUnknown != bUnknown {
				return aUnknown
			}
			if reverse {
				return a.getSize() < b.getSize()
			}
			return a.getSize() > b.getSize()
		})
	case randomSort:
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		r.Shuffle(len(items), func(i, j int) {
			items[i], items[j] = items[j], items[i]
		})
	}
	for i := range items {
		if selectedItem == items[i] {
			settings.cursor = i
			break
		}
	}
}

func hasUnknownDirectorySize(it item) bool {
	if !it.isDirectory() {
		return false
	}
	if file, ok := it.(*filepathItem); ok {
		return file.sizeStr == ""
	}
	_, isDrive := it.(*driveItem)
	return !isDrive
}
