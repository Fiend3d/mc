package main

import (
	"path/filepath"
	"sort"
	"strings"
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
				result = false
			} else if !iIsDir && jIsDir {
				result = true
			} else {
				if jIsDir {
					a := page.items[i].getName()
					b := page.items[j].getName()
					result = strings.ToLower(a) < strings.ToLower(b)
				} else {
					a := filepath.Ext(page.items[i].getName())
					b := filepath.Ext(page.items[j].getName())
					// TODO: handle same extensions
					result = strings.ToLower(a) < strings.ToLower(b)
				}
			}
			if reverse {
				result = !result
			}
			return result
		})
	case modifiedTimeSort:
	case normalSort:
	case randomSort:
	case sizeSort:
	}
}
