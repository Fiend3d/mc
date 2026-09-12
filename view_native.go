package main

import (
	"fmt"
	"image/color"

	"github.com/Fiend3d/catatui"
	"github.com/Fiend3d/catatui/widgets"
	"github.com/dustin/go-humanize"
	"mc/internal/paint"
	"mc/shutil"
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
	contentHeight := area.Height
	if m.taskStripVisible() {
		contentHeight--
	}
	content := catatui.NewRect(0, 0, area.Width, contentHeight)
	m.input.SetWidth(max(1, m.width-4))
	m.pathInput.SetWidth(max(1, m.width-4))
	if m.taskView {
		m.drawTasks(f, content)
		if m.taskStripVisible() {
			m.drawTaskStrip(f, catatui.NewRect(0, area.Height-1, area.Width, 1))
		}
		return
	}
	switch m.mode {
	case compareMode:
		m.drawCompare(f, content)
	case helpMode, helpFilterMode, messagesMode, bookmarksMode, tabsMode, searchMode, gitListMode:
		savedWidth, savedHeight := m.width, m.height
		m.width = int(area.Width)
		m.height = int(content.Height)
		var text string
		switch m.mode {
		case helpMode, helpFilterMode:
			// The help body does not reach the scrollbar lane or its gutter, so
			// give the whole area the body background first. Left on the
			// frame-wide empty style they read as a stripe down the edge.
			f.Buffer().SetStyle(content, m.theme.baseStyle.Native())
			// Wrap two columns narrower: one for the scrollbar's lane and one
			// as a gutter so long lines do not touch it. The title still
			// centres over the whole width so it looks centred on screen.
			m.helpChrome = min(2, int(area.Width)-1)
			m.width = max(1, int(area.Width)-m.helpChrome)
			text = viewHelp(m)
		case messagesMode:
			text = viewMessages(m)
		case bookmarksMode:
			text = viewBookmarks(m)
		case tabsMode:
			text = viewTabs(m)
		case searchMode:
			text = viewSearch(m)
		case gitListMode:
			text = viewGitList(m)
		}
		textAt(f, content, text)
		if m.mode == helpMode || m.mode == helpFilterMode {
			m.drawHelpScrollbar(f, content)
		}
		m.width, m.height = savedWidth, savedHeight
	default:
		for i, p := range m.panes {
			x, w := 0, left
			if i == 1 {
				x = left + 1
				w = int(area.Width) - x
			}
			m.drawPane(f, catatui.NewRect(uint16(x), 0, uint16(w), content.Height), p, i, i == m.activePane)
		}
		for y := uint16(0); y < content.Height; y++ {
			f.Buffer().SetString(uint16(left), y, "│", m.theme.emptyStyle.Foreground(m.theme.grayColor).Native())
		}
		m.drawMode(f, area)
	}
	if m.taskStripVisible() {
		m.drawTaskStrip(f, catatui.NewRect(0, area.Height-1, area.Width, 1))
	}
	if m.quitting {
		m.dialog(f, area, "Unfinished tasks", "Cancel tasks and quit?\n\ny / Enter: cancel and quit    n / Esc: keep working")
	}
}

var modeLabels = map[mode]string{jumpMode: "JUMP", filterMode: "FILTER", renameMode: "RENAME", createMode: "CREATE", pathMode: "PATH", shellMode: "SHELL", transferMode: "TRANSFER"}

func (m *model) drawPane(f *catatui.Frame, a catatui.Rect, p *pane, paneIndex int, active bool) {
	style := m.theme.emptyStyle
	accent := m.theme.grayColor
	if active {
		accent = m.theme.accentColor3
	}
	if !p.hasTabs() {
		m.drawEmptyPane(f, a, style, accent, paneIndex, active)
		return
	}
	t := p.tabs[p.currentTab]
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
	pathWidth, summary := gitPathLane(t, int(a.Width), active && m.mode == pathMode)
	// The two lanes of the row never light up together. A breadcrumb component
	// is measured before the path is truncated, so a long path leaves cut-off
	// components sitting under the summary's columns; without this they would
	// highlight from a pointer that is nowhere near them.
	pathHover, summaryHover := -1, -1
	if m.hoverPathPane == paneIndex {
		if m.hoverPathX < pathWidth {
			pathHover = m.hoverPathX
		} else {
			summaryHover = m.hoverPathX
		}
	}
	if active && m.mode == pathMode {
		path = m.pathInput.View()
	} else {
		hoverX := pathHover
		rootEnd := -1
		if t.git != nil {
			// The root is a prefix of the directory by construction, so its
			// length is where the component that names the repository ends.
			rootEnd = len(t.git.root)
		}
		path = colorizeDirRoot(path,
			style.Bold(true).Foreground(m.theme.whiteColor),
			style.Bold(true).Foreground(m.theme.accentColor5),
			style.Bold(true).Foreground(m.theme.whiteColor),
			style.Bold(true).Foreground(m.gitRootColor(t.git)),
			rootEnd, pathWidth, hoverX)
	}
	textAt(f, catatui.NewRect(a.X, 1, a.Width, 1), path)
	if summary != "" {
		// Right-aligned, so it stays put while the path grows and shrinks
		// underneath it.
		lane := catatui.NewRect(a.X+uint16(pathWidth), 1, a.Width-uint16(pathWidth), 1)
		textAt(f, lane, style.Width(int(lane.Width)).Align(paint.Right).Render(m.gitSummary(t, int(a.Width), summaryHover)))
	}
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
			// The git column is only taken when the directory is in a
			// repository, so listings elsewhere keep every column they have.
			git := ""
			if t.git != nil {
				state := gitNone
				if file, ok := it.(*filepathItem); ok {
					state = file.git
				}
				git = s.Bold(true).Foreground(m.gitColor(state)).Render(state.letter()) + s.Render(" ")
				if state != gitNone {
					nameStyle = s.Foreground(m.gitColor(state))
				}
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
			nameWidth := max(1, int(a.Width)-3-paint.Width(git)-paint.Width(metadata))
			line = action + cursor + mark + git + paint.PlaceHorizontal(nameWidth, paint.Left, truncate(name, nameWidth)) + metadata
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
		if label := modeLabels[m.mode]; label != "" {
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
				if file, ok := it.(*filepathItem); ok && file.git != gitNone {
					footer += "  " + file.git.name()
				}
				if drive, ok := it.(*driveItem); ok {
					footer = fmt.Sprintf("%s free / %s · %s", humanize.Bytes(drive.free), humanize.Bytes(drive.total), drive.driveType)
				}
			}
		}
	}
	textAt(f, catatui.NewRect(a.X, a.Height-1, a.Width, 1), footer)
}

// gitPathLane splits the path row in two: the breadcrumb keeps the left, the
// repository summary takes the right. A pane with no room for both keeps the
// path whole -- knowing where you are outranks knowing the branch -- and so
// does one whose path row has been taken over by Path mode.
func gitPathLane(t *tab, width int, editing bool) (int, string) {
	if t.git == nil || editing {
		return width, ""
	}
	summary := t.git.summary()
	if summary == "" {
		return width, ""
	}
	lane := paint.Width(summary) + 1
	if width-lane < gitPathMin {
		return width, ""
	}
	return width - lane, summary
}

// gitPathMin is the narrowest the breadcrumb may be squeezed to before the
// summary gives up its lane.
const gitPathMin = 24

// gitSummaryStart is the column the summary's first character lands on: the
// lane is right-aligned, so the text hugs the end of the row. Hit testing and
// rendering both measure from here, and it is the one place the alignment is
// assumed.
func gitSummaryStart(t *tab, width int) int {
	return width - paint.Width(t.git.summary())
}

// gitSummaryAtX reports the tally whose text covers x on the path row, the way
// paneTabAtX reports the tab under a click. gitNone means nothing clickable is
// there -- the branch, an arrow, or somewhere else on the row entirely.
func gitSummaryAtX(t *tab, width, x int) gitState {
	if _, summary := gitPathLane(t, width, false); summary == "" {
		return gitNone // no lane on this row, so nothing on it to hit
	}
	position := gitSummaryStart(t, width)
	for _, segment := range t.git.segments() {
		segmentWidth := paint.Width(segment.text)
		if segment.tally != gitNone && x >= position && x < position+segmentWidth {
			return segment.tally
		}
		position += segmentWidth
	}
	return gitNone
}

// gitSummary renders the summary segment by segment so the tally under the
// pointer can light up on its own. It lights up the way a breadcrumb component
// does -- the text turns white, the background stays where it is -- because
// both are the same gesture: pointing at something on the path row that a
// click will act on.
func (m *model) gitSummary(t *tab, width, hoverX int) string {
	base := m.theme.emptyStyle.Foreground(m.gitRootColor(t.git))
	position := gitSummaryStart(t, width)
	var out strings.Builder
	for _, segment := range t.git.segments() {
		segmentWidth := paint.Width(segment.text)
		style := base
		if segment.tally != gitNone && hoverX >= position && hoverX < position+segmentWidth {
			style = m.theme.emptyStyle.Bold(true).Foreground(m.theme.whiteColor)
		}
		out.WriteString(style.Render(segment.text))
		position += segmentWidth
	}
	return out.String()
}

// gitRootColor colours the component of the path that names the repository,
// and the summary beside it: a work tree with changes in it takes an accent of
// its own, a clean one goes green. The path's own colour is accentColor5, so
// the mark has to come from somewhere else to be seen at all.
func (m *model) gitRootColor(info *gitInfo) color.Color {
	if info.dirty() {
		return m.theme.accentColor1
	}
	return m.theme.greenColor
}

// gitColor gives each state its colour: what is staged reads green, changed
// work yellow, trouble red, and anything git is not tracking stays gray.
func (m *model) gitColor(state gitState) color.Color {
	switch state {
	case gitConflicted, gitDeleted:
		return m.theme.redColor
	case gitModified:
		return m.theme.accentColor5
	case gitAdded, gitRenamed:
		return m.theme.greenColor
	case gitUntracked:
		return m.theme.accentColor2
	}
	// Ignored entries, and anything git says nothing about, stay quiet.
	return m.theme.grayColor
}

// drawEmptyPane paints a pane that holds no tabs. It keeps the pane's half of
// the screen so the split never moves, and says how to fill it back up.
func (m *model) drawEmptyPane(f *catatui.Frame, a catatui.Rect, style paint.Style, accent color.Color, paneIndex int, active bool) {
	for row := uint16(0); row < a.Height; row++ {
		textAt(f, catatui.NewRect(a.X, a.Y+row, a.Width, 1), style.Width(int(a.Width)).Render(""))
	}
	hints := []string{"no tabs", "", "T restores the last closed tab", "gg opens a path", "Tab switches panes"}
	if paneIndex == 1 {
		// Only this pane is written to tabs.list on exit.
		hints = append(hints, "", "tabs opened here are saved for the next launch")
	}
	top := a.Y + a.Height/2 - uint16(len(hints)/2)
	for i, hint := range hints {
		row := top + uint16(i)
		if row >= a.Y+a.Height-2 {
			break
		}
		hintStyle := style.Foreground(m.theme.grayColor)
		if i == 0 {
			hintStyle = style.Bold(true).Foreground(accent)
		}
		textAt(f, catatui.NewRect(a.X, row, a.Width, 1),
			hintStyle.Width(int(a.Width)).Align(paint.Center).Render(truncate(hint, int(a.Width))))
	}
	// Path mode is the way out of an empty pane, so it has to be visible here
	// too, on the row a populated pane uses for its path bar.
	if active && m.mode == pathMode {
		textAt(f, catatui.NewRect(a.X, a.Y+1, a.Width, 1), m.pathInput.View())
	}
	if active && m.mode != normalMode {
		if label := modeLabels[m.mode]; label != "" {
			textAt(f, catatui.NewRect(a.X, a.Height-2, a.Width, 1),
				style.Foreground(accent).Width(int(a.Width)).Render(label))
		}
	}
	footer := ""
	if active && m.ticks > 0 && len(m.log) > 0 {
		footer = m.log[len(m.log)-1].render(m.theme, false)
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
		m.dialog(f, a, "Go", "g  Change path      m  Modified files\nt  Tabs             u  Untracked files\nT  Theme            a  Added files\nc  Config directory\nC  Save config\ns  Calculate size")
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

// taskLabel names the work itself: the command, prefixed by the action when it
// is being undone or redone.
func taskLabel(t *task) string {
	if t.action == "undo" || t.action == "redo" {
		return fmt.Sprintf("%s %s", t.action, t.cmd)
	}
	return fmt.Sprint(t.cmd)
}

// taskProgress is the measured half of a summary -- percentages and counts --
// returned in pieces so the task list can give them a column of their own.
func taskProgress(t *task) []string {
	p := t.progress
	var parts []string
	if p.Total > 0 {
		parts = append(parts, fmt.Sprintf("%.0f%% %s/%s", min(100, float64(p.Bytes)*100/float64(p.Total)), humanize.Bytes(uint64(p.Bytes)), humanize.Bytes(uint64(p.Total))))
	} else if p.Bytes > 0 || p.Files > 0 {
		// A size walk cannot know its byte total in advance, so it reports the
		// running counts here and takes its percentage from the step count.
		parts = append(parts, fmt.Sprintf("%s in %d files", humanize.Bytes(uint64(p.Bytes)), p.Files))
	}
	if p.TotalFiles > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d files", p.Files, p.TotalFiles))
	}
	if p.Total == 0 && p.TotalSteps > 0 {
		parts = append(parts, fmt.Sprintf("%.0f%%", min(100, float64(p.Steps)*100/float64(p.TotalSteps))))
	}
	return parts
}

// taskSummary is the one-line form the strip uses, where a single task has the
// whole row to itself.
func taskSummary(t *task) string {
	s := fmt.Sprintf("#%d %s · %s", t.id, taskLabel(t), t.state)
	for _, part := range taskProgress(t) {
		s += " · " + part
	}
	return s
}

// taskRatio reports how full the gauge should be, and whether the task can be
// measured at all. Transfers divide bytes by a known total; a size walk has no
// byte total, so it falls back to finished top-level entries -- coarse, but it
// advances for real.
// drawHelpScrollbar draws the help viewport's scrollbar down the reserved
// right-hand column. Help that fits on one screen gets no scrollbar at all.
func (m *model) drawHelpScrollbar(f *catatui.Frame, content catatui.Rect) {
	viewport := int(content.Height) - 1 // the last row is the filter prompt
	if viewport <= 0 || m.helpLines <= viewport || content.Width == 0 {
		return
	}
	// The widget derives its extent from contentLength-1+viewport, so it wants
	// the number of scroll positions rather than the number of lines. Passing
	// that makes the extent the real line count and lands the thumb flush with
	// the bottom of the track at maximum scroll.
	state := widgets.NewScrollbarState(m.maxHelpScroll() + 1).
		Position(m.help).
		ViewportContentLength(viewport)
	// Track and thumb share the body background: differing backgrounds make the
	// column change colour as the thumb resizes and moves.
	bar := widgets.NewScrollbar(widgets.ScrollbarVerticalRight).
		TrackStyle(m.theme.baseStyle.Foreground(m.theme.grayColor).Native()).
		ThumbStyle(m.theme.baseStyle.Foreground(m.theme.accentColor3).Native()).
		BeginSymbolNone().
		EndSymbolNone()
	area := catatui.NewRect(content.X, content.Y, content.Width, uint16(viewport))
	catatui.RenderStatefulWidgetOn(f, bar, area, &state)
}

func taskRatio(p shutil.Progress) (float64, bool) {
	switch {
	case p.Total > 0 && !p.Scanning:
		return min(1, float64(p.Bytes)/float64(p.Total)), true
	case p.TotalSteps > 0:
		return min(1, float64(p.Steps)/float64(p.TotalSteps)), true
	}
	return 0, false
}

func (m *model) drawTaskStrip(f *catatui.Frame, a catatui.Rect) {
	// File work owns the strip because it has a real gauge; a read-only scan
	// takes it only when nothing is being modified. The rest are counted.
	var primary, readOnly *task
	active := 0
	for _, t := range m.taskList {
		if t.state != "running" && t.state != "scanning" && t.state != "cancelling" {
			continue
		}
		active++
		if t.readOnly {
			if readOnly == nil {
				readOnly = t
			}
		} else if primary == nil {
			primary = t
		}
	}
	if primary == nil {
		primary = readOnly
	}
	if primary == nil {
		return
	}
	text := taskSummary(primary)
	if active > 1 {
		text += fmt.Sprintf(" · +%d more", active-1)
	}
	ratio, measured := taskRatio(primary.progress)
	if measured {
		f.RenderWidget(widgets.NewGauge().Ratio(ratio).
			Label(truncate(text, int(a.Width))).
			GaugeStyle(m.theme.baseStyle.Foreground(m.theme.accentColor3).Native()), a)
		return
	}
	textAt(f, a, m.theme.baseStyle.Foreground(m.theme.accentColor3).Width(int(a.Width)).Render(text))
}

// taskStateColumn is the width of the state column, sized to "cancelling", the
// longest state a task can be in.
const taskStateColumn = 10

// taskStateColor tells the states apart at a glance: work in flight takes the
// accent, a failure red, a finished task green, anything dormant gray.
func (m *model) taskStateColor(state string) color.Color {
	switch state {
	case "running", "scanning":
		return m.theme.accentColor3
	case "cancelling":
		return m.theme.accentColor4
	case "completed":
		return m.theme.greenColor
	case "failed":
		return m.theme.redColor
	}
	return m.theme.grayColor
}

// taskHeader counts the list the way the tabs and bookmarks headers do, and
// says which slice of it is on screen when the list is longer than the view.
func taskHeader(tasks []*task, start, shown int) string {
	if len(tasks) == 0 {
		return " no tasks in this session"
	}
	word := "tasks"
	if len(tasks) == 1 {
		word = "task"
	}
	active := 0
	for _, t := range tasks {
		switch t.state {
		case "running", "scanning", "cancelling":
			active++
		}
	}
	header := fmt.Sprintf(" %d %s", len(tasks), word)
	if active > 0 {
		header += fmt.Sprintf(" · %d active", active)
	}
	if shown < len(tasks) {
		header += fmt.Sprintf(" · showing %d-%d", start+1, start+shown)
	}
	return header
}

// taskRow lays out one task: the cursor, its number, what it is doing, the
// state in a column of its own and the figures trailing in gray. Every row
// shares the columns, so the states read straight down the list.
func (m *model) taskRow(t *task, selected bool, idWidth, labelWidth, width int) string {
	style := m.theme.baseStyle
	cursor := "   "
	if selected {
		style, cursor = m.theme.cursorStyle, " > "
	}
	row := style.Bold(true).Render(cursor)
	row += style.Foreground(m.theme.grayColor).Width(idWidth + 1).Render(fmt.Sprintf("[%d]", t.id))
	row += style.Width(labelWidth + 1).Render(truncate(taskLabel(t), labelWidth))
	row += style.Bold(true).Foreground(m.taskStateColor(t.state)).Width(taskStateColumn + 1).Render(t.state)
	rest := max(0, width-len(cursor)-idWidth-labelWidth-taskStateColumn-3)
	row += style.Foreground(m.theme.grayColor).Width(rest).Render(truncate(strings.Join(taskProgress(t), " · "), rest))
	return row
}

// taskDetail spells out where the selected task has got to: the path it
// reached, or why it stopped.
func (m *model) taskDetail() (string, bool) {
	if m.taskCursor < 0 || m.taskCursor >= len(m.taskList) {
		return "", false
	}
	t := m.taskList[m.taskCursor]
	if t.err != nil {
		return " " + t.err.Error(), true
	}
	return " " + t.progress.Path, false
}

// drawTasks lists the session's tasks in the shape of the other overlays: a
// counted header, a scrolling list under a cursor, then the selected task's
// detail and the keys that act on it.
func (m *model) drawTasks(f *catatui.Frame, a catatui.Rect) {
	width, height := int(a.Width), int(a.Height)
	empty := m.theme.emptyStyle
	gray := empty.Foreground(m.theme.grayColor)

	// The list leaves the last two rows to the detail and the keys, and scrolls
	// to hold the cursor near the middle once the tasks no longer fit.
	rows := max(1, height-3)
	start := 0
	if len(m.taskList) > rows {
		start = min(max(0, m.taskCursor-rows/2), len(m.taskList)-rows)
	}
	idWidth, labelWidth := 3, 0
	for _, t := range m.taskList {
		idWidth = max(idWidth, len(fmt.Sprintf("[%d]", t.id)))
		labelWidth = max(labelWidth, paint.Width(taskLabel(t)))
	}
	// Half the row is as much as a command name may take: the figures after it
	// are what the list is opened for.
	labelWidth = min(labelWidth, max(8, width/2))

	var list strings.Builder
	drawn := 0
	for i := start; i < len(m.taskList) && drawn < rows; i, drawn = i+1, drawn+1 {
		list.WriteString(m.taskRow(m.taskList[i], i == m.taskCursor, idWidth, labelWidth, width))
		list.WriteRune('\n')
	}
	for pad := drawn; pad < rows; pad++ {
		list.WriteString(empty.Width(width).Render(" "))
		list.WriteRune('\n')
	}

	var s strings.Builder
	header := truncate(taskHeader(m.taskList, start, drawn), width)
	s.WriteString(empty.Width(width).Bold(true).Foreground(m.theme.accentColor3).Render(header))
	s.WriteRune('\n')
	s.WriteString(list.String())

	detail, failed := m.taskDetail()
	detailStyle := gray
	if failed {
		detailStyle = empty.Foreground(m.theme.redColor)
	}
	s.WriteString(detailStyle.Width(width).Render(truncate(detail, width)))
	s.WriteRune('\n')

	help := gray.Render(" Keys:")
	if m.taskCursor >= 0 && m.taskCursor < len(m.taskList) {
		switch m.taskList[m.taskCursor].state {
		case "queued", "running", "scanning":
			help += empty.Render(" c ")
			help += gray.Render("- cancel")
		}
	}
	help += empty.Render(" Esc ")
	help += gray.Render("- return")
	s.WriteString(gray.Width(width).Render(truncate(help, width)))

	textAt(f, a, s.String())
}

func (m *model) dimensions() {
	m.width = max(1, (m.screenWidth-1)/2)
	if m.activePane == 1 {
		m.width = max(1, m.screenWidth-m.width-1)
	}
	reservedRows := 0
	if m.taskStripVisible() {
		reservedRows = 1
	}
	m.height = max(4, m.screenHeight-1-reservedRows)
	switch m.mode {
	case helpMode, helpFilterMode, messagesMode, bookmarksMode, tabsMode, searchMode, gitListMode:
		m.width = max(1, m.screenWidth)
		m.height = max(4, m.screenHeight-reservedRows)
	}
}
