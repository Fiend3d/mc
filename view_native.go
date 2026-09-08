package main

import (
	"fmt"
	"github.com/Fiend3d/catatui"
	"github.com/Fiend3d/catatui/widgets"
	"github.com/dustin/go-humanize"
	"mc/internal/paint"
	"strings"
)

func textAt(f *catatui.Frame, area catatui.Rect, s string) {
	f.RenderWidget(widgets.NewParagraphFromText(catatui.NewText(paint.Lines(s)...)), area)
}
func (m *model) draw(f *catatui.Frame) {
	area := f.Area()
	f.Buffer().SetStyle(area, m.theme.emptyStyle.Native())
	if area.Width < 40 || area.Height < 8 {
		textAt(f, area, "Resize terminal to at least 40 × 8")
		return
	}
	left := (int(area.Width) - 1) / 2
	m.dimensions()
	m.input.SetWidth(max(1, m.width-4))
	m.pathInput.SetWidth(max(1, m.width-4))
	if m.taskView {
		m.drawTasks(f, area)
		m.drawTaskStrip(f, catatui.NewRect(0, area.Height-1, area.Width, 1))
		return
	}
	switch m.mode {
	case helpMode, helpFilterMode, messagesMode, bookmarksMode, tabsMode, searchMode:
		savedWidth, savedHeight := m.width, m.height
		m.width = int(area.Width)
		m.height = int(area.Height) - 1
		var text string
		switch m.mode {
		case helpMode, helpFilterMode:
			text = viewHelp(m)
		case messagesMode:
			text = viewMessages(m)
		case bookmarksMode:
			text = viewBookmarks(m)
		case tabsMode:
			text = viewTabs(m)
		case searchMode:
			text = viewSearch(m)
		}
		textAt(f, catatui.NewRect(0, 0, area.Width, area.Height-1), text)
		m.width, m.height = savedWidth, savedHeight
	default:
		for i, p := range m.panes {
			x, w := 0, left
			if i == 1 {
				x = left + 1
				w = int(area.Width) - x
			}
			m.drawPane(f, catatui.NewRect(uint16(x), 0, uint16(w), area.Height-1), p, i, i == m.activePane)
		}
		for y := uint16(0); y < area.Height-1; y++ {
			f.Buffer().SetString(uint16(left), y, "│", m.theme.emptyStyle.Foreground(m.theme.grayColor).Native())
		}
		m.drawMode(f, area)
	}
	m.drawTaskStrip(f, catatui.NewRect(0, area.Height-1, area.Width, 1))
	if m.quitting {
		m.dialog(f, area, "Unfinished tasks", "Cancel tasks and quit?\n\ny / Enter: cancel and quit    n / Esc: keep working")
	}
}
func (m *model) drawPane(f *catatui.Frame, a catatui.Rect, p *pane, paneIndex int, active bool) {
	t := p.tabs[p.currentTab]
	style := m.theme.emptyStyle
	accent := m.theme.grayColor
	if active {
		accent = m.theme.accentColor3
	}
	labels := paneTabLabels(p)
	var tabLine strings.Builder
	baseTabStyle := style.Foreground(accent).Bold(active)
	for i, label := range labels {
		if i > 0 {
			tabLine.WriteString(style.Render(" "))
		}
		tabStyle := baseTabStyle
		if m.hoverTabPane == paneIndex && m.hoverTabIndex == i {
			tabStyle = m.theme.selectionStyle.Foreground(accent).Bold(active)
		}
		tabLine.WriteString(tabStyle.Render(label))
	}
	textAt(f, catatui.NewRect(a.X, a.Y, a.Width, 1), truncate(tabLine.String(), int(a.Width)))
	path := t.dir
	if path == "" {
		path = "This PC"
	}
	if active && m.mode == pathMode {
		path = m.pathInput.View()
	} else {
		hoverX := -1
		if m.hoverPathPane == paneIndex {
			hoverX = m.hoverPathX
		}
		path = colorizeDirHover(path, style.Bold(true).Foreground(m.theme.whiteColor), style.Bold(true).Foreground(m.theme.accentColor5), style.Bold(true).Foreground(m.theme.whiteColor), int(a.Width), hoverX)
	}
	textAt(f, catatui.NewRect(a.X, 1, a.Width, 1), path)
	rows := int(a.Height) - 4
	settings := t.getPageSettings()
	items := t.page.getItems()
	settings.update(len(items))
	settings.start = min(settings.start, max(0, len(items)-rows))
	for row := 0; row < rows; row++ {
		index := settings.start + row
		s := m.theme.emptyStyle
		line := ""
		if index < len(items) {
			s = m.theme.baseStyle
			it := items[index]
			if it.isSelected() {
				s = m.theme.selectionStyle
			}
			if m.hoverPane == paneIndex && m.hoverIndex == index {
				if it.isSelected() {
					s = m.theme.cursorStyle
				} else {
					s = m.theme.selectionStyle
				}
			}
			cursor := " "
			if index == settings.cursor {
				cursor = "›"
				if active {
					s = m.theme.cursorStyle
				}
			}
			action := " "
			switch it.getAction() {
			case itemActionCopy:
				action = s.Foreground(m.theme.accentColor4).Render("┃")
			case itemActionCut:
				action = s.Foreground(m.theme.accentColor2).Render("┃")
			}
			mark := " "
			if it.isSelected() {
				mark = s.Foreground(m.theme.whiteColor).Render("┃")
			}
			name := strings.Map(func(r rune) rune {
				if r < 32 || r == 127 {
					return '�'
				}
				return r
			}, it.getName())
			nameStyle := s.Foreground(m.theme.whiteColor)
			if it.isDirectory() {
				nameStyle = s.Foreground(m.theme.accentColor4)
			} else if strings.HasSuffix(strings.ToLower(it.getName()), ".exe") {
				nameStyle = s.Foreground(m.theme.greenColor)
			}
			name = nameStyle.Render(name)
			if it.isDirectory() {
				name += s.Bold(true).Render("/")
			}
			metadata := ""
			if a.Width >= 42 {
				if it.isDirectory() {
					if file, ok := it.(*filepathItem); ok && file.sizeStr != "" {
						metadata = " " + file.sizeStr
					} else {
						metadata = " <DIR>"
					}
				} else {
					metadata = " " + humanize.Bytes(it.getSize())
				}
			}
			if a.Width >= 65 {
				metadata += s.Foreground(m.theme.grayColor).Render(" " + it.getModTime().Format("02.01.06 15:04"))
			}
			nameWidth := max(1, int(a.Width)-3-paint.Width(metadata))
			line = action + cursor + mark + paint.PlaceHorizontal(nameWidth, paint.Left, truncate(name, nameWidth)) + metadata
		} else if items == nil && row == 0 {
			line = "Loading…"
		}
		textAt(f, catatui.NewRect(a.X, uint16(row+2), a.Width, 1), s.Width(int(a.Width)).Render(line))
	}
	selected := 0
	var size uint64
	for _, it := range items {
		if it.isSelected() {
			selected++
			size += it.getSize()
		}
	}
	status := fmt.Sprintf("%d items · %d selected %s", len(items), selected, humanize.Bytes(size))
	if active && m.mode != normalMode {
		labels := map[mode]string{jumpMode: "JUMP", filterMode: "FILTER", renameMode: "RENAME", createMode: "CREATE", pathMode: "PATH", shellMode: "SHELL", transferMode: "TRANSFER"}
		if label := labels[m.mode]; label != "" {
			status = label + " · " + status
		}
	}
	textAt(f, catatui.NewRect(a.X, a.Height-2, a.Width, 1), style.Foreground(accent).Render(status))
	footer := ""
	if active {
		switch m.mode {
		case filterMode, renameMode, createMode, shellMode, transferMode:
			footer = m.input.View()
		default:
			if m.ticks > 0 && len(m.log) > 0 {
				footer = m.log[len(m.log)-1].render(m.theme, false)
			} else if len(items) > 0 {
				it := items[settings.cursor]
				footer = it.getName() + "  " + it.getExtra()
				if drive, ok := it.(*driveItem); ok {
					footer = fmt.Sprintf("%s free / %s · %s", humanize.Bytes(drive.free), humanize.Bytes(drive.total), drive.driveType)
				}
			}
		}
	}
	textAt(f, catatui.NewRect(a.X, a.Height-1, a.Width, 1), footer)
}
func filepathBase(path string) string {
	path = strings.TrimRight(path, "\\/")
	if i := strings.LastIndexAny(path, "\\/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func paneTabLabels(p *pane) []string {
	labels := make([]string, 0, len(p.tabs))
	for i, t := range p.tabs {
		name := t.dir
		if name == "" {
			name = "This PC"
		}
		name = filepathBase(name)
		if i == p.currentTab {
			name = "[" + name + "]"
		}
		labels = append(labels, name)
	}
	return labels
}

func paneTabAtX(p *pane, x int) int {
	position := 0
	for i, label := range paneTabLabels(p) {
		if i > 0 {
			position++
		}
		width := paint.Width(label)
		if x >= position && x < position+width {
			return i
		}
		position += width
	}
	return -1
}
func (m *model) dialog(f *catatui.Frame, a catatui.Rect, title, body string) {
	w := min(int(a.Width)-2, max(30, paint.Width(body)+4))
	h := min(int(a.Height)-2, len(strings.Split(body, "\n"))+2)
	r := catatui.NewRect(uint16((int(a.Width)-w)/2), uint16((int(a.Height)-h)/2), uint16(w), uint16(h))
	f.RenderWidget(widgets.Clear{}, r)
	f.RenderWidget(widgets.NewParagraphFromText(catatui.NewText(paint.Lines(body)...)).Style(m.theme.baseStyle.Native()).Block(widgets.Bordered().Title(title)), r)
}
func (m *model) drawMode(f *catatui.Frame, a catatui.Rect) {
	switch m.mode {
	case confirmDialogMode:
		detail := ""
		switch m.cmd.(type) {
		case *deleteCommand:
			detail = "Permanent deletion. This cannot be undone.\n"
		case *fileActionCommand:
			detail = "Replace existing destinations. This cannot be undone.\n"
		}
		choice := "Yes    [No]"
		if m.yes {
			choice = "[Yes]    No"
		}
		m.dialog(f, a, "Confirm", fmt.Sprintf("%s\n%s\n%s\ny: confirm    n / Esc: cancel", m.cmd, detail, choice))
	case goMode:
		m.dialog(f, a, "Go", "g  Change path\nt  Tabs\nT  Theme\nc  Config directory\nC  Save config\ns  Calculate size")
	case sortMode:
		m.dialog(f, a, "Sort", "m  Modified time    a  Alphabetical\nn  Normal           e  Extension\ns  Size             r  Random\nUppercase reverses order")
	case copyMode:
		m.dialog(f, a, "Copy names", "c/C  File paths      d/D  Directory\nf    Filenames       n    Without extension\na/A  Path arguments  s    Filename arguments\nq/Q  Path array      w    Filename array")
	case themeMode:
		var lines []string
		for i, t := range themeList {
			prefix := "  "
			if i == m.themeCursor {
				prefix = "› "
			}
			lines = append(lines, prefix+t.name)
		}
		m.dialog(f, a, "Theme", strings.Join(lines, "\n"))
	case transferMode:
		action := "Copy"
		if m.transferMove {
			action = "Move"
		}
		m.dialog(f, a, action+" to directory", m.input.View()+"\nEnter: submit    Esc: cancel")
	}
}
func taskSummary(t *task) string {
	p := t.progress
	action := ""
	if t.action == "undo" || t.action == "redo" {
		action = t.action + " "
	}
	s := fmt.Sprintf("#%d %s%s · %s", t.id, action, t.cmd, t.state)
	if p.Total > 0 {
		s += fmt.Sprintf(" · %.0f%% %s/%s", min(100, float64(p.Bytes)*100/float64(p.Total)), humanize.Bytes(uint64(p.Bytes)), humanize.Bytes(uint64(p.Total)))
	}
	if p.TotalFiles > 0 {
		s += fmt.Sprintf(" · %d/%d files", p.Files, p.TotalFiles)
	}
	return s
}
func (m *model) drawTaskStrip(f *catatui.Frame, a catatui.Rect) {
	text := ""
	for _, t := range m.taskList {
		if t.state == "running" || t.state == "scanning" || t.state == "cancelling" {
			text = taskSummary(t)
			if t.progress.Total > 0 && !t.progress.Scanning {
				ratio := min(1, float64(t.progress.Bytes)/float64(t.progress.Total))
				f.RenderWidget(widgets.NewGauge().Ratio(ratio).
					Label(truncate(text, int(a.Width))).
					GaugeStyle(m.theme.baseStyle.Foreground(m.theme.accentColor3).Native()), a)
				return
			}
			break
		}
	}
	if text != "" {
		textAt(f, a, m.theme.baseStyle.Foreground(m.theme.accentColor3).Width(int(a.Width)).Render(text))
	}
}
func (m *model) drawTasks(f *catatui.Frame, a catatui.Rect) {
	textAt(f, catatui.NewRect(0, 0, a.Width, 1), m.theme.baseStyle.Bold(true).Render("Tasks · c: cancel · Esc: return"))
	start := max(0, m.taskCursor-(int(a.Height)-4)/2)
	for row, i := 1, start; i < len(m.taskList) && row < int(a.Height)-2; i, row = i+1, row+1 {
		t := m.taskList[i]
		style := m.theme.baseStyle
		if i == m.taskCursor {
			style = m.theme.cursorStyle
		}
		textAt(f, catatui.NewRect(0, uint16(row), a.Width, 1), style.Width(int(a.Width)).Render(taskSummary(t)))
	}
	if len(m.taskList) == 0 {
		textAt(f, catatui.NewRect(0, 2, a.Width, 1), "No tasks in this session")
	}
	if m.taskCursor >= 0 && m.taskCursor < len(m.taskList) {
		t := m.taskList[m.taskCursor]
		detail := t.progress.Path
		if t.err != nil {
			detail = t.err.Error()
		}
		textAt(f, catatui.NewRect(0, a.Height-2, a.Width, 1), detail)
	}
}

func (m *model) dimensions() {
	m.width = max(1, (m.screenWidth-1)/2)
	if m.activePane == 1 {
		m.width = max(1, m.screenWidth-m.width-1)
	}
	m.height = max(4, m.screenHeight-2)
	switch m.mode {
	case helpMode, helpFilterMode, messagesMode, bookmarksMode, tabsMode, searchMode:
		m.width = max(1, m.screenWidth)
		m.height = max(4, m.screenHeight-1)
	}
}
