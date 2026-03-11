package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type mode int

const (
	normalMode mode = iota
	hiddenMode
	visualMode
	helpMode
	helpFilterMode
	goMode
	confirmDialogMode
	confirmDialogVisualMode
	jumpMode
	messagesMode
	tabsMode
	filterMode
	sortMode
	renameMode
	createMode
	pathMode
	copyMode
	copyVisualMode
	bookmarksMode
	searchMode
	shellMode
)

type model struct {
	cfg *Config

	err        error
	tabs       []*tab
	currentTab int
	closedTabs []string
	mode       mode
	visual     int
	width      int
	height     int
	click      mouseClick

	help       int
	helpFilter string

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

	bm     *bookmarks
	search *search

	theme *theme

	result string
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

func (m *model) getStartEnd() (int, int) {
	settings := m.getTab().getPageSettings()
	start := min(settings.cursor, m.visual)
	end := max(settings.cursor, m.visual)
	return start, end
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
	if settings.cursor < settings.start {
		settings.start = settings.cursor
		return
	}
	actualHeight := m.height - 4
	if settings.cursor > settings.start+actualHeight {
		settings.start = settings.cursor - actualHeight
	}
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
	if m.mode == searchMode {
		i, _ := m.search.mapIndex(m.search.cursor)
		item := m.search.items[i]
		return []string{item.path}
	}
	items := m.getPage().getItems()
	if len(items) == 0 {
		return nil
	}
	var paths []string
	switch m.mode {
	case visualMode, copyVisualMode:
		start, end := m.getStartEnd()
		for i := start; i <= end; i++ {
			paths = append(paths, items[i].getFullPath())
		}
	default:
		settings := m.getTab().getPageSettings()
		for i := range items {
			if items[i].isSelected() {
				paths = append(paths, items[i].getFullPath())
			}
		}
		if len(paths) == 0 {
			paths = append(paths, items[settings.cursor].getFullPath())
		}
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

	return fmt.Sprintf("%d paths %s", len(paths), txt)
}

func (m *model) confirm(cmd command) {
	if m.mode == visualMode {
		m.mode = confirmDialogVisualMode
	} else {
		m.mode = confirmDialogMode
	}
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

func (m *model) addCommand(cmd command) tea.Cmd {
	return tea.Batch(
		m.addMessage(msgInfo, fmt.Sprintf("command: %s", cmd)),
		m.spinner.Tick,
		m.execute(cmd),
	)
}

type tickMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m *model) fillPage(tab int, items []item) error {
	m.tabs[tab].page.items = items
	m.tabs[tab].filter()
	return nil
}

func (m *model) addMessage(msgType msgType, msg string) tea.Cmd {
	message := newMessage(msgType, msg)
	m.log = append(m.log, message)
	m.ticks += 6
	return tick()
}

func (m *model) left() (tea.Model, tea.Cmd) {
	tab := m.getTab()
	parent := filepathDir(tab.dir)
	tab.set(parent)
	return m, m.readDir(m.currentTab, parent)
}

func (m *model) right() (tea.Model, tea.Cmd) {
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
		return m, tea.ExecProcess(cmd, nil)
	}
	if !selectedItem.isDirectory() {
		return m, nil
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

func (m *model) getTab() *tab {
	return m.tabs[m.currentTab]
}

func (m *model) getPage() *page { // probably redundant
	tab := m.getTab()
	return tab.page
}

func newTextinput(t *theme) textinput.Model {
	input := textinput.New()
	input.Prompt = " > "
	input.CharLimit = 255 // hello, windows!
	input.SetWidth(0)     // TODO: it's bugged right now
	styles := input.Styles()
	styles.Cursor.Shape = tea.CursorBar
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
	return input
}

func initialModel(dirs []string) model {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("warning: failed to load config: %s\n\n", err.Error())
	}

	tabs := make([]*tab, len(dirs))
	for i, dir := range dirs {
		tabs[i] = newTab(dir, &page{})
	}

	theme := newTheme()
	input := newTextinput(theme)
	pathInput := newTextinput(theme)
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = theme.baseStyle.Foreground(theme.accentColor1)

	return model{
		cfg:        cfg,
		tabs:       tabs,
		currentTab: 0,
		mode:       normalMode,
		theme:      theme,
		input:      input,
		pathInput:  pathInput,
		spinner:    s,
		cm:         newCommandManager(),
	}
}
