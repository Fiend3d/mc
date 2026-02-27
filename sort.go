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
	switch method {
	case alphabeticSort:
		page := m.getPage()
		sort.Slice(page.items, func(i, j int) bool {
			result := strings.ToLower(page.items[i].getName()) < strings.ToLower(page.items[j].getName())
			if reverse {
				result = !result
			}
			return result
		})
	case extensionSort:
		page := m.getPage()
		sort.Slice(page.items, func(i, j int) bool {
			result := true
			iIsDir := page.items[i].isDirectory()
			jIsDir := page.items[j].isDirectory()
			if iIsDir && !jIsDir {
				result = true
			} else if !iIsDir && jIsDir {
				result = false
			} else {
				if jIsDir {
					a := page.items[i].getName()
					b := page.items[j].getName()
					result = strings.ToLower(a) < strings.ToLower(b)
				} else {
					iHasExt := hasExtension(page.items[i].getName())
					jHasExt := hasExtension(page.items[j].getName())
					if iHasExt && !jHasExt {
						result = true
					} else if !iHasExt && jHasExt {
						result = false
					} else if !iHasExt && !jHasExt {
						a := page.items[i].getName()
						b := page.items[j].getName()
						result = strings.ToLower(a) < strings.ToLower(b)
					} else {
						a := filepath.Ext(page.items[i].getName())
						b := filepath.Ext(page.items[j].getName())
						if a == b {
							a := page.items[i].getName()
							b := page.items[j].getName()
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
		page := m.getPage()
		sort.Slice(page.items, func(i, j int) bool {
			a := page.items[i]
			b := page.items[j]
			result := a.getModTime().After(b.getModTime())
			if reverse {
				result = !result
			}
			return result
		})
	case normalSort:
		page := m.getPage()
		sort.Slice(page.items, func(i, j int) bool {
			result := true
			a := page.items[i]
			b := page.items[j]
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
		page := m.getPage()
		sort.Slice(page.items, func(i, j int) bool {
			a := page.items[i]
			b := page.items[j]
			result := a.getSize() > b.getSize()
			if reverse {
				result = !result
			}
			return result
		})
	case randomSort:
		page := m.getPage()
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		r.Shuffle(len(page.items), func(i, j int) {
			page.items[i], page.items[j] = page.items[j], page.items[i]
		})
	}
}
