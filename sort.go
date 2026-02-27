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

func (m *model) sort(method sortMethod, reverse bool) {
	items := m.getPage().getItems()
	switch method {
	case alphabeticSort:
		sort.Slice(items, func(i, j int) bool {
			result := strings.ToLower(items[i].getName()) < strings.ToLower(items[j].getName())
			if reverse {
				result = !result
			}
			return result
		})
	case extensionSort:
		sort.Slice(items, func(i, j int) bool {
			result := true
			iIsDir := items[i].isDirectory()
			jIsDir := items[j].isDirectory()
			if iIsDir && !jIsDir {
				result = true
			} else if !iIsDir && jIsDir {
				result = false
			} else {
				if jIsDir {
					a := items[i].getName()
					b := items[j].getName()
					result = strings.ToLower(a) < strings.ToLower(b)
				} else {
					iHasExt := hasExtension(items[i].getName())
					jHasExt := hasExtension(items[j].getName())
					if iHasExt && !jHasExt {
						result = true
					} else if !iHasExt && jHasExt {
						result = false
					} else if !iHasExt && !jHasExt {
						a := items[i].getName()
						b := items[j].getName()
						result = strings.ToLower(a) < strings.ToLower(b)
					} else {
						a := filepath.Ext(items[i].getName())
						b := filepath.Ext(items[j].getName())
						if a == b {
							a := items[i].getName()
							b := items[j].getName()
							result = strings.ToLower(a) < strings.ToLower(b)
						} else {
							result = strings.ToLower(a) < strings.ToLower(b)
						}
					}
				}
			}
			if reverse {
				result = !result
			}
			return result
		})
	case modifiedTimeSort:
		sort.Slice(items, func(i, j int) bool {
			a := items[i]
			b := items[j]
			result := a.getModTime().After(b.getModTime())
			if reverse {
				result = !result
			}
			return result
		})
	case normalSort:
		sort.Slice(items, func(i, j int) bool {
			result := true
			a := items[i]
			b := items[j]
			iIsDir := a.isDirectory()
			jIsDir := b.isDirectory()
			if iIsDir && !jIsDir {
				result = true
			} else if !iIsDir && jIsDir {
				result = false
			} else {
				result = strings.ToLower(a.getName()) < strings.ToLower(b.getName())
			}
			if reverse {
				result = !result
			}
			return result
		})
	case sizeSort:
		sort.Slice(items, func(i, j int) bool {
			a := items[i]
			b := items[j]
			result := a.getSize() > b.getSize()
			if reverse {
				result = !result
			}
			return result
		})
	case randomSort:
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		r.Shuffle(len(items), func(i, j int) {
			items[i], items[j] = items[j], items[i]
		})
	}
}
