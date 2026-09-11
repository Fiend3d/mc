package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mc/internal/event"
	"mc/shutil"
	"mc/widgets/spinner"
	"mc/widgets/textinput"
)

type mode int

const (
	normalMode mode = iota
	hiddenMode
	helpMode
	helpFilterMode
	goMode
	confirmDialogMode
	jumpMode
	messagesMode
	tabsMode
	filterMode
	sortMode
	renameMode
	createMode
	pathMode
	copyMode
	bookmarksMode
	searchMode
	gitListMode
	shellMode
	themeMode
	transferMode
)

type pane struct {
	tabs       []*tab
	currentTab int
	closedTabs []string
}

type model struct {
	*pane
	panes                     [2]*pane
	activePane                int
	screenWidth, screenHeight int
	taskList                  []*task
	taskWorkers               *sync.WaitGroup
	taskCursor                int
	taskView                  bool
	taskEvents                chan event.Msg
	transferPaths             []string
	transferMove              bool
	quitting                  bool
	quitResult                bool

	cfg *Config

	err                         error
	mode                        mode
	width                       int
	height                      int
	click                       mouseClick
	hoverPane, hoverIndex       int
	hoverSearchIndex            int
	hoverTabPane, hoverTabIndex int
	hoverPathPane, hoverPathX   int

	help         int
	helpLines    int // total help lines from the last render, for scroll clamping
	helpChrome   int // columns reserved beside the text for the scrollbar lane
	helpDragging bool
	helpFilter   string

	yes bool
	cmd command

	jobs        int
	confirmQuit bool
	spinner     spinner.Model

	cm *commandManager

	pathInput    textinput.Model
	pathInputDir string // to optimize autocomplete
	input        textinput.Model

	shellHistory        []string
	shellHistoryCurrent int

	renamePaths []string

	tabsCursor int
	tabsStart  int

	log      []message
	logStart int
	ticks    int

	bm       *bookmarks
	search   *search
	gitList  gitList
	hoverGitIndex int

	theme       *theme
	themeOld    *theme
	themeCursor int

	result string
}

// helpViewport is how many documentation lines fit on screen; the last row
// belongs to the filter prompt.
func (m *model) helpViewport() int { return max(0, m.height-1) }

// maxHelpScroll is the furthest offset that still shows content, so the help
// cannot be scrolled off into blank space.
func (m *model) maxHelpScroll() int { return max(0, m.helpLines-m.helpViewport()) }

// scrollHelp moves the help offset by delta, clamped to the length measured by
// the last render. Help that has not been drawn yet has no length to clamp
// against, so the offset moves freely and viewHelp corrects it on the next
// frame rather than swallowing the keypress.
func (m *model) scrollHelp(delta int) int {
	next := max(0, m.help+delta)
	if m.helpLines == 0 {
		return next
	}
	return min(next, m.maxHelpScroll())
}

// helpScrollbarColumn is the screen column the help scrollbar occupies. The
// help view wraps one column narrower to leave it free.
func (m *model) helpScrollbarColumn() int { return m.screenWidth - 1 }

// helpThumbLength mirrors the scrollbar widget's thumb sizing so a dragged
// cursor row can be mapped back to a scroll offset.
func (m *model) helpThumbLength() int {
	track := m.helpViewport()
	extent := m.maxHelpScroll() + track
	if track <= 0 || extent <= 0 {
		return max(track, 0)
	}
	length := (track*track + extent/2) / extent
	return min(max(length, 1), track)
}

// helpOffsetForRow maps a row of the scrollbar track to a scroll offset,
// centring the thumb on the cursor so a grabbed thumb tracks the pointer.
func (m *model) helpOffsetForRow(row int) int {
	track := m.helpViewport()
	span := track - m.helpThumbLength()
	scroll := m.maxHelpScroll()
	if span <= 0 || scroll <= 0 {
		return 0
	}
	start := min(max(row-m.helpThumbLength()/2, 0), span)
	return min((start*scroll+span/2)/span, scroll)
}

// helpScrollbarHit reports whether a click at x, y landed on the scrollbar.
func (m *model) helpScrollbarHit(x, y int) bool {
	return m.helpLines > m.helpViewport() &&
		x == m.helpScrollbarColumn() &&
		y >= 0 && y < m.helpViewport()
}

func (m *model) addJob() {
	m.jobs++
}

func (m *model) jobDone() {
	m.jobs--
	if m.jobs == 0 {
		m.confirmQuit = false
	}
}

func (m *model) hasJobs() bool {
	return m.jobs > 0
}

func (m *model) resetInput(placeholder string) {
	m.input.Placeholder = placeholder
	m.input.ShowSuggestions = false
	m.input.Reset()
	m.input.Focus()
}

func (m *model) setFilter() {
	patterns := strings.FieldsFunc(m.input.Value(), func(r rune) bool {
		return r == ',' || r == ';'
	})
	tab := m.getTab()
	tab.filterText = patterns
}

func (p *page) getItems() []item {
	if p.isTemp() {
		return p.tempItems
	} else {
		return p.items
	}
}

func (p *page) length() int {
	if p.isTemp() {
		return len(p.tempItems)
	}
	return len(p.items)
}

func (p *page) isTemp() bool {
	return p.tempItems != nil
}

func (m *model) updateStart() {
	settings := m.getTab().getPageSettings()
	revealCursor(settings, max(1, m.height-3))
}

func revealCursor(settings *pageSettings, rows int) {
	if settings.cursor < settings.start {
		settings.start = settings.cursor
		return
	}
	actualHeight := rows - 1
	if settings.cursor > settings.start+actualHeight {
		settings.start = settings.cursor - actualHeight
	}
}

func (m *model) clearHover() {
	m.clearItemHover()
	m.hoverPathPane, m.hoverPathX = -1, -1
}

// clearItemHover drops only the hover that is an index into something: a row,
// a tab, a search hit. The path row's hover is an x on the breadcrumb, so a
// listing that reloads underneath it has no reason to take the highlight away
// while the pointer has not moved.
func (m *model) clearItemHover() {
	m.hoverPane, m.hoverIndex, m.hoverSearchIndex = -1, -1, -1
	m.hoverTabPane, m.hoverTabIndex, m.hoverGitIndex = -1, -1, -1
}

func (m *model) updateTabsStart() {
	if m.tabsCursor < m.tabsStart {
		m.tabsStart = m.tabsCursor
		return
	}
	actualHeight := m.height - 3
	if m.tabsCursor > m.tabsStart+actualHeight {
		m.tabsStart = m.tabsCursor - actualHeight
	}
}

func (m *model) moveCursor(move int) {
	tab := m.getTab()
	settings := tab.getPageSettings()
	settings.cursor += move
	length := tab.page.length()
	if settings.cursor >= length {
		settings.cursor = length - 1
	}
	if settings.cursor < 0 {
		settings.cursor = 0
	}
	m.updateStart()
}

type msgType int

type message struct {
	time        time.Time
	messageType msgType
	message     string
}

func newMessage(messageType msgType, msg string) message {
	return message{time.Now(), messageType, msg}
}

func (m *message) render(theme *theme, renderTime bool) string {
	var s strings.Builder
	if renderTime {
		timeStyle := theme.emptyStyle.Foreground(theme.grayColor)
		s.WriteString(timeStyle.Render(m.time.Format("02.01.2006 15:04")))
		s.WriteString(timeStyle.Render(" "))
	}
	style := &theme.emptyStyle
	switch m.messageType {
	case msgTxt:
		s.WriteString(style.Render(m.message))
	case msgInfo:
		s.WriteString(style.Foreground(theme.accentColor3).Render("[info] "))
		s.WriteString(style.Render(m.message))
	case msgWarning:
		s.WriteString(style.Foreground(theme.accentColor4).Render("[warning] "))
		s.WriteString(style.Render(m.message))
	case msgError:
		s.WriteString(style.Foreground(theme.accentColor1).Render("[error] "))
		s.WriteString(style.Render(m.message))
	case msgDone:
		s.WriteString(style.Foreground(theme.greenColor).Render("[done] "))
		s.WriteString(style.Render(m.message))
	case msgFail:
		s.WriteString(style.Foreground(theme.redColor).Render("[fail] "))
		s.WriteString(style.Render(m.message))

	}
	return s.String()
}

func (m *model) getPaths() []string {
	if m.mode == gitListMode {
		if entry := m.gitList.current(); entry != nil {
			return []string{entry.path}
		}
		return nil
	}
	if m.mode == searchMode {
		i, _ := m.search.mapIndex(m.search.cursor)
		if i < 0 || i >= len(m.search.items) {
			return nil
		}
		item := m.search.items[i]
		return []string{item.path}
	}
	items := m.getPage().getItems()
	if len(items) == 0 {
		return nil
	}
	var paths []string
	settings := m.getTab().getPageSettings()
	for _, it := range items {
		if it.isSelected() {
			paths = append(paths, it.getFullPath())
		}
	}
	if len(paths) == 0 && settings.cursor < len(items) {
		paths = append(paths, items[settings.cursor].getFullPath())
	}
	return paths
}

func (m *model) copyCut(cut bool) string {
	paths := m.getPaths()
	var txt string
	if cut {
		setClipboardFiles(paths, OpCut)
		txt = "cut"
	} else {
		setClipboardFiles(paths, OpCopy)
		txt = "copied"
	}
	invalidateClipboardCache() // the refresh right after this must see it

	return fmt.Sprintf("%d paths %s", len(paths), txt)
}

func (m *model) confirm(cmd command) {
	m.mode = confirmDialogMode
	m.yes = false
	m.cmd = cmd
}

const (
	msgTxt msgType = iota
	msgInfo
	msgWarning
	msgError
	msgDone
	msgFail
)

func (m *model) addCommand(cmd command) event.Cmd {
	return event.Batch(
		m.addMessage(msgInfo, fmt.Sprintf("command: %s", cmd)),
		m.spinner.Tick,
		m.execute(cmd),
	)
}

type tickMsg struct{}

func tick() event.Cmd {
	return event.Tick(time.Second, func(time.Time) event.Msg {
		return tickMsg{}
	})
}

func (m *model) addMessage(msgType msgType, msg string) event.Cmd {
	message := newMessage(msgType, msg)
	m.log = append(m.log, message)
	m.ticks += 6
	return tick()
}

func (m *model) left() (event.Model, event.Cmd) {
	tab := m.getTab()
	dir := tab.dir
	parent := filepathDir(tab.dir)
	tab.set(parent)
	return m, event.Sequence(
		m.readDir(m.currentTab, parent),
		selectItem(m.getTab(), dir),
	)
}

func (m *model) right(addNewTab bool) (event.Model, event.Cmd) {
	tab := m.getTab()
	settings := tab.getPageSettings()
	items := tab.page.getItems()
	if settings.cursor > len(items)-1 {
		return m, nil
	}
	selectedItem := items[settings.cursor]
	if !selectedItem.isDirectory() &&
		strings.HasSuffix(strings.ToUpper(selectedItem.getName()), ".EXE") { // probably a mistake
		cmd := exec.Command(selectedItem.getFullPath())
		cmd.Dir = tab.dir
		return m, runCmd(cmd, tab.dir)
	}
	if !selectedItem.isDirectory() {
		return m, nil
	}
	if addNewTab {
		dir := selectedItem.getFullPath()
		tabCopy := newTab(dir, &page{})
		m.tabs = append(m.tabs, tabCopy)
		m.currentTab = len(m.tabs) - 1
		return m, event.Batch(
			m.addMessage(msgInfo, fmt.Sprintf("%s opened in a new tab", dir)),
			m.readDir(m.currentTab, dir),
		)
	}
	if tab.page.isTemp() {
		for i := range tab.page.items {
			if tab.page.items[i] == selectedItem {
				settings.cursor = i
				break
			}
		}
	}
	dir := selectedItem.getFullPath()
	tab.set(dir)
	return m, m.readDir(m.currentTab, dir)
}

// saveTabs remembers the right pane for the next launch. Errors are dropped:
// quitting must not fail over this, and the UI is already gone.
func (m *model) saveTabs() {
	right := m.panes[1]
	dirs := make([]string, 0, len(right.tabs))
	for _, t := range right.tabs {
		dirs = append(dirs, t.dir)
	}
	_ = saveTabs(dirs)
}

func (m *model) getTab() *tab {
	return m.tabs[m.currentTab]
}

func (m *model) getPage() *page { // probably redundant
	tab := m.getTab()
	return tab.page
}

func setTextinputStyle(input *textinput.Model, t *theme) {
	styles := input.Styles()
	styles.Cursor.Shape = event.CursorBar
	styles.Focused.Text = t.emptyStyle
	styles.Focused.Placeholder = t.emptyStyle.Foreground(t.grayColor)
	styles.Focused.Suggestion = t.emptyStyle.Foreground(t.grayColor)
	styles.Focused.Prompt = t.emptyStyle.Bold(true).Foreground(t.greenColor)

	styles.Blurred.Text = t.emptyStyle
	styles.Blurred.Placeholder = t.emptyStyle.Foreground(t.grayColor)
	styles.Blurred.Suggestion = t.emptyStyle.Foreground(t.grayColor)
	styles.Blurred.Prompt = t.emptyStyle.Foreground(t.grayColor)

	styles.Cursor.Color = t.emptyStyle.GetForeground()
	styles.Cursor.Blink = true
	input.SetStyles(styles)
}

func setSpinnerStyle(s *spinner.Model, t *theme) {
	s.Spinner = spinner.Dot
	s.Style = t.baseStyle.Foreground(t.accentColor1)
}

func newTextinput(t *theme) textinput.Model {
	input := textinput.New()
	input.Prompt = " > "
	input.CharLimit = 255 // hello, windows!
	input.SetWidth(0)     // TODO: it's bugged right now
	setTextinputStyle(&input, t)
	return input
}

func initialModel(dirs []string) model {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("warning: failed to load config: %s\n\n", err.Error())
	}

	if len(dirs) == 0 {
		wd, _ := os.Getwd()
		dirs = []string{wd}
	}
	for i, dir := range dirs {
		if dir != "" {
			if abs, err := filepath.Abs(dir); err == nil {
				dirs[i] = abs
			}
		}
	}
	leftDirs := append([]string{dirs[0]}, dirs[min(2, len(dirs)):]...)
	tabs := make([]*tab, len(leftDirs))
	for i, dir := range leftDirs {
		tabs[i] = newTab(dir, &page{})
	}

	theme := newTheme(cfg.Theme)
	input := newTextinput(theme)
	pathInput := newTextinput(theme)
	s := spinner.New()
	setSpinnerStyle(&s, theme)

	// A single directory opens on the left only; the right pane starts empty
	// rather than showing the same thing twice, and carries over the tabs it
	// held last session unless an argument names its directory.
	left := &pane{tabs: tabs}
	right := &pane{}
	if len(dirs) > 1 {
		right.tabs = []*tab{newTab(dirs[1], &page{})}
	} else {
		saved, _ := loadTabs()
		for _, dir := range saved {
			if dir != "" && !shutil.DirExists(dir) {
				continue // the folder is gone, or its drive is unplugged
			}
			right.tabs = append(right.tabs, newTab(dir, &page{}))
		}
	}
	return model{
		pane: left, panes: [2]*pane{left, right}, taskEvents: make(chan event.Msg, 128), taskWorkers: &sync.WaitGroup{},
		hoverPane: -1, hoverIndex: -1, hoverSearchIndex: -1,
		hoverTabPane: -1, hoverTabIndex: -1, hoverPathPane: -1, hoverPathX: -1,
		hoverGitIndex: -1,
		cfg:       cfg,
		mode:      normalMode,
		theme:     theme,
		input:     input,
		pathInput: pathInput,
		spinner:   s,
		cm:        newCommandManager(),
	}
}
