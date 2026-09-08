package main

// rangeSelection remembers selections that predate the current gesture.
type rangeSelection struct {
	anchor   int
	selected map[string]bool
}

func (m *model) finishRangeSelection() {
	for _, p := range m.panes {
		for _, t := range p.tabs {
			t.page.selectionRange = nil
		}
	}
}

func rangeKey(key string) bool {
	switch key {
	case "shift+up", "shift+down", "shift+home", "shift+end":
		return true
	}
	return false
}

func (m *model) selectRangeTo(end int) {
	t := m.getTab()
	items := t.page.getItems()
	if len(items) == 0 {
		return
	}
	s := t.getPageSettings()
	s.update(len(items))
	if t.page.selectionRange == nil {
		r := &rangeSelection{anchor: s.cursor, selected: make(map[string]bool)}
		for _, it := range items {
			r.selected[it.getFullPath()] = it.isSelected()
		}
		t.page.selectionRange = r
	}
	r := t.page.selectionRange
	end = max(0, min(end, len(items)-1))
	lo, hi := min(r.anchor, end), max(r.anchor, end)
	for i, it := range items {
		it.setSelected(r.selected[it.getFullPath()] || (i >= lo && i <= hi))
	}
	s.cursor = end
	m.updateStart()
}
