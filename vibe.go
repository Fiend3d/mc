package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"mc/internal/event"
)

type vibeRow struct {
	id, parent, path, text  string
	kind                    byte // d directory, f file, h hunk, l line, i information
	depth, file, hunk, line int
	branch                  bool
}
type vibeState struct {
	root                               string
	tempPath                           string
	generation                         uint64
	cancel                             context.CancelFunc
	viewerCancel                       context.CancelFunc
	loading, pending, viewing, invalid bool
	snapshot                           vibeSnapshot
	rows                               []vibeRow
	visual                             []vibeVisualRow
	rowOffsets                         []int
	layoutWidth                        int
	noWrap, scrollbarDragging          bool
	hover                              string
	all                                []vibeRow
	collapsed                          map[string]bool
	cursor, start                      int
	err                                string
	viewerErr                          string
}
type vibeDoneMsg struct {
	generation uint64
	snapshot   vibeSnapshot
	err        error
}

func (v *vibeState) stop() {
	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}
}
func (v *vibeState) stopViewer() {
	if v.viewerCancel != nil {
		v.viewerCancel()
		v.viewerCancel = nil
	}
}
func (m *model) openVibe() (event.Model, event.Cmd) {
	if !m.cfg.Git {
		return m, m.addMessage(msgWarning, "Vibe requires git = true in config.toml")
	}
	if gitBinary() == "" {
		return m, m.addMessage(msgWarning, "Vibe requires Git on PATH")
	}
	root := findRepoRoot(m.currentDir())
	if root == "" {
		return m, m.addMessage(msgWarning, "Vibe: not inside a Git repository")
	}
	generation := m.vibe.generation + 1
	m.vibe.stop()
	m.vibe.stopViewer()
	m.vibe = vibeState{root: root, generation: generation, collapsed: map[string]bool{}}
	m.vibe.snapshot.root = root
	m.vibe.rebuild()
	m.mode = vibeMode
	return m, m.refreshVibe()
}

// Runtime calls this every two seconds while visible. Refresh requests are
// coalesced while a worker runs; external viewers pause the refresh lifecycle.
func (m *model) refreshVibe() event.Cmd {
	v := &m.vibe
	if m.mode != vibeMode || v.viewing {
		return nil
	}
	if v.loading {
		v.pending = true
		return nil
	}
	v.generation++
	generation, root := v.generation, v.root
	ctx, cancel := context.WithCancel(context.Background())
	v.cancel = cancel
	v.loading = true
	v.pending = false
	return func() event.Msg {
		defer cancel()
		snapshot, err := readVibe(ctx, root)
		return vibeDoneMsg{generation, snapshot, err}
	}
}
func (m *model) applyVibe(msg vibeDoneMsg) event.Cmd {
	v := &m.vibe
	if m.mode != vibeMode || msg.generation != v.generation {
		return nil
	}
	v.stop()
	v.loading = false
	if !v.invalid && !v.viewing {
		if msg.err != nil {
			v.err = msg.err.Error()
		} else {
			v.err = ""
			if msg.snapshot.fingerprint != v.snapshot.fingerprint {
				v.replace(msg.snapshot, m.vibeHeight())
			}
		}
	}
	v.invalid = false
	if v.pending && !v.viewing {
		return m.refreshVibe()
	}
	return nil
}
func (m *model) vibeHeight() int {
	h := m.screenHeight - 3
	if m.taskStripVisible() {
		h--
	}
	return max(1, h)
}
func (v *vibeState) current() *vibeRow {
	if v.cursor < 0 || v.cursor >= len(v.rows) {
		return nil
	}
	return &v.rows[v.cursor]
}
func (v *vibeState) keep(height int) {
	v.cursor = min(max(0, v.cursor), max(0, len(v.rows)-1))
	position := v.rowPosition(v.cursor)
	if position < v.start {
		v.start = position
	}
	if position >= v.start+height {
		v.start = position - height + 1
	}
	v.clampView(height)
}
func (v *vibeState) move(delta, height int) { v.cursor += delta; v.keep(height) }

func (v *vibeState) clampView(height int) {
	v.cursor = min(max(0, v.cursor), max(0, len(v.rows)-1))
	count := len(v.rows)
	if v.visual != nil {
		count = len(v.visual)
	}
	v.start = min(max(0, v.start), max(0, count-height))
}

func (v *vibeState) scroll(delta, height int) {
	v.start += delta
	v.clampView(height)
}

func (v *vibeState) page(delta, height int) {
	if len(v.visual) == 0 {
		v.move(delta, height)
		return
	}
	position := min(max(0, v.rowPosition(v.cursor)+delta), len(v.visual)-1)
	v.cursor = v.visual[position].row
	v.scroll(delta, height)
}

func (v *vibeState) replace(snapshot vibeSnapshot, height int) {
	var old vibeRow
	parents := []string{}
	if row := v.current(); row != nil {
		old = *row
		parent := row.parent
		for parent != "" {
			parents = append(parents, parent)
			next := ""
			for _, r := range v.all {
				if r.id == parent {
					next = r.parent
					break
				}
			}
			parent = next
		}
	}
	offset := v.rowPosition(v.cursor) - v.start
	v.snapshot = snapshot
	v.rebuild()
	index := -1
	for i, r := range v.rows {
		if r.id == old.id {
			index = i
			break
		}
	}
	// Identical line text survives inserted lines above it. Prefer the
	// nearest matching line when the file contains repeated text.
	if index < 0 && old.kind == 'l' {
		best := int(^uint(0) >> 1)
		for i, r := range v.rows {
			if r.kind == 'l' && r.path == old.path && r.text == old.text {
				distance := r.line - old.line
				if distance < 0 {
					distance = -distance
				}
				if distance < best {
					best = distance
					index = i
				}
			}
		}
	}
	if index < 0 {
		for _, parent := range parents {
			for i, r := range v.rows {
				if r.id == parent {
					index = i
					break
				}
			}
			if index >= 0 {
				break
			}
		}
	}
	if index < 0 {
		index = 0
	}
	v.cursor = index
	v.start = max(0, v.rowPosition(index)-offset)
	v.clampView(height)
}

func (v *vibeState) rebuild() {
	// Construct an explicit directory hierarchy, then flatten in display
	// order. The root exists even when there are no changed files.
	dirs := map[string][]string{"": {}}
	linked := map[string]bool{}
	files := map[string][]int{}
	for i, f := range v.snapshot.files {
		dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(f.path)))
		if dir == "." {
			dir = ""
		}
		files[dir] = append(files[dir], i)
		for d := dir; d != ""; {
			if linked[d] {
				break
			}
			linked[d] = true
			parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(d)))
			if parent == "." {
				parent = ""
			}
			dirs[parent] = append(dirs[parent], d)
			d = parent
		}
	}
	v.all = nil
	var walk func(string, string, int)
	walk = func(dir, parent string, depth int) {
		id := "d:" + dir
		label := filepath.Base(filepath.FromSlash(dir))
		if dir == "" {
			label = v.root
		}
		v.all = append(v.all, vibeRow{id: id, parent: parent, path: dir, text: label, kind: 'd', depth: depth, branch: dir != "", file: -1})
		sort.Strings(dirs[dir])
		for _, child := range dirs[dir] {
			walk(child, id, depth+1)
		}
		sort.Slice(files[dir], func(i, j int) bool {
			return v.snapshot.files[files[dir][i]].path < v.snapshot.files[files[dir][j]].path
		})
		for _, fi := range files[dir] {
			f := v.snapshot.files[fi]
			fid := "f:" + f.path
			v.all = append(v.all, vibeRow{id: fid, parent: id, path: f.path, text: filepath.Base(filepath.FromSlash(f.path)), kind: 'f', depth: depth + 1, file: fi, branch: len(f.hunks) > 0 || f.note != ""})
			occurrences := map[[32]byte]int{}
			for hi, h := range f.hunks {
				var changes strings.Builder
				for _, line := range h.lines {
					if line.kind == '+' || line.kind == '-' {
						changes.WriteByte(line.kind)
						changes.WriteString(line.text)
						changes.WriteByte('\n')
					}
				}
				signature := sha256.Sum256([]byte(changes.String()))
				hid := fmt.Sprintf("h:%s:%x:%d", f.path, signature, occurrences[signature])
				occurrences[signature]++
				v.all = append(v.all, vibeRow{id: hid, parent: fid, path: f.path, text: h.header, kind: 'h', depth: depth + 2, file: fi, hunk: hi, branch: true})
				for li, l := range h.lines {
					lid := fmt.Sprintf("l:%s:%c:%d:%d:%x", f.path, l.kind, l.old, l.new, sha256.Sum256([]byte(l.text)))
					v.all = append(v.all, vibeRow{id: lid, parent: hid, path: f.path, text: string(l.kind) + l.text, kind: 'l', depth: depth + 3, file: fi, hunk: hi, line: li})
				}
			}
			if f.note != "" {
				v.all = append(v.all, vibeRow{id: "i:" + f.path, parent: fid, path: f.path, text: f.note, kind: 'i', depth: depth + 2, file: fi})
			}
		}
	}
	walk("", "", 0)
	v.visible()
}
func (v *vibeState) visible() {
	v.rows = nil
	hiddenDepth := -1
	for _, row := range v.all {
		if hiddenDepth >= 0 && row.depth > hiddenDepth {
			continue
		}
		hiddenDepth = -1
		v.rows = append(v.rows, row)
		if row.branch && v.collapsed[row.id] {
			hiddenDepth = row.depth
		}
	}
	if v.layoutWidth > 0 {
		v.layout(v.layoutWidth)
	}
}
func (v *vibeState) toggle(height int) {
	if r := v.current(); r != nil && r.branch {
		v.collapsed[r.id] = !v.collapsed[r.id]
		v.visible()
		v.keep(height)
	}
}
func (m *model) handleVibe(key string) (event.Model, event.Cmd) {
	v := &m.vibe
	h := m.vibeHeight()
	row := v.current()
	switch key {
	case "esc", "q":
		v.stop()
		v.stopViewer()
		v.generation++
		v.loading = false
		v.pending = false
		v.snapshot = vibeSnapshot{}
		v.rows = nil
		v.all = nil
		m.mode = normalMode
	case "f5":
		return m, m.refreshVibe()
	case "w":
		v.toggleWrap(h)
	case "j", "down":
		v.move(1, h)
	case "k", "up":
		v.move(-1, h)
	case "pgdown":
		v.page(h, h)
	case "pgup":
		v.page(-h, h)
	case "home":
		v.cursor = 0
		v.keep(h)
	case "end":
		v.cursor = len(v.rows) - 1
		v.keep(h)
	case "space":
		v.toggle(h)
	case "e":
		selected := ""
		if row != nil {
			selected = row.id
		}
		v.collapsed = map[string]bool{}
		v.visible()
		for i, r := range v.rows {
			if r.id == selected {
				v.cursor = i
				break
			}
		}
		v.keep(h)
	case "c":
		for _, r := range v.all {
			if r.branch {
				v.collapsed[r.id] = true
			}
		}
		v.visible()
		v.cursor, v.start = 0, 0
		v.hover = ""
	case "h", "left":
		if row != nil {
			if row.branch && !v.collapsed[row.id] {
				v.toggle(h)
			} else {
				for i, r := range v.rows {
					if r.id == row.parent {
						v.cursor = i
						v.keep(h)
						break
					}
				}
			}
		}
	case "l", "right":
		if row != nil && row.branch {
			if v.collapsed[row.id] {
				v.toggle(h)
			} else {
				v.move(1, h)
			}
		}
	case "]", "[":
		if v.layoutWidth != max(1, m.screenWidth-2) {
			v.layout(max(1, m.screenWidth-2))
		}
		step := 1
		if key == "[" {
			step = -1
		}
		// Navigate all hunks, opening ancestors when a target is collapsed.
		start := -1
		if row != nil {
			for i, r := range v.all {
				if r.id == row.id {
					start = i
					break
				}
			}
		}
		for i := start + step; i >= 0 && i < len(v.all); i += step {
			target := v.all[i]
			if target.kind != 'h' {
				continue
			}
			for _, r := range v.all {
				if r.branch && (r.id == "d:" || r.id == "f:"+target.path || r.id == target.id || (r.kind == 'd' && strings.HasPrefix(target.path, r.path+"/"))) {
					delete(v.collapsed, r.id)
				}
			}
			v.visible()
			for j, r := range v.rows {
				if r.id == target.id {
					v.cursor = j
					// Put the hunk header at the top so its changed lines are
					// visible too, rather than leaving the header at the bottom.
					v.start = v.rowPosition(j)
					v.clampView(h)
					break
				}
			}
			break
		}
	case "enter":
		if row != nil && row.kind == 'd' {
			v.toggle(h)
		} else {
			return m, m.viewVibe()
		}
	case "f3":
		return m, m.viewVibe()
	}
	return m, nil
}

func (m *model) updateVibe(msg event.Msg) (bool, event.Cmd) {
	switch e := msg.(type) {
	case vibeDoneMsg:
		return true, m.applyVibe(e)
	case vibeViewerReadyMsg:
		return true, m.startVibeViewer(e)
	case vibeViewerDoneMsg:
		if m.mode == vibeMode && e.generation == m.vibe.generation {
			m.vibe.tempPath = ""
			m.vibe.viewing = false
			if e.err != nil {
				m.vibe.viewerErr = e.err.Error()
			}
			return true, m.refreshVibe()
		}
		return true, nil
	}
	if m.mode != vibeMode {
		return false, nil
	}
	switch e := msg.(type) {
	case event.KeyMsg:
		_, cmd := m.handleVibe(e.String())
		return true, cmd
	case event.MouseWheelMsg:
		m.vibe.hover = ""
		if !m.taskView && !m.quitting {
			steps := 3
			if e.Button == event.MouseWheelUp {
				steps = -3
			}
			m.vibe.scroll(steps, m.vibeHeight())
		}
		return true, nil
	case event.MouseClickMsg:
		if !m.taskView && !m.quitting && e.Button == event.MouseLeft {
			if e.X == m.screenWidth-1 && e.Y >= 2 && e.Y < 2+m.vibeHeight() {
				m.vibe.scrollbarDragging = true
				m.vibe.scrollbarTo(e.Y-2, m.vibeHeight())
				return true, nil
			}
			index := m.vibe.rowAtY(e.Y, m.vibeHeight())
			if index >= 0 {
				m.vibe.cursor = index
				m.vibe.clampView(m.vibeHeight())
				m.click = newClick(e.X, e.Y, &m.click)
				if m.click.doubleClick {
					r := m.vibe.current()
					if r.branch {
						m.vibe.toggle(m.vibeHeight())
					} else {
						return true, m.viewVibe()
					}
				}
			}
		}
		return true, nil
	case event.MouseHoverMsg:
		m.vibe.hover = ""
		if e.Index >= 0 && e.Index < len(m.vibe.rows) {
			m.vibe.hover = m.vibe.rows[e.Index].id
		}
		return true, nil
	case event.MouseDragMsg:
		if m.vibe.scrollbarDragging {
			m.vibe.scrollbarTo(e.Y-2, m.vibeHeight())
		}
		return true, nil
	case event.MouseUpMsg:
		m.vibe.scrollbarDragging = false
		return true, nil
	case event.MouseTabMsg, event.PasteMsg:
		return true, nil
	}
	return false, nil
}
