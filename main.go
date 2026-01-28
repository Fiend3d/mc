package main

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"
)

type mode int

const (
	normal mode = iota
	visual
	shell
)

type itemType int

const (
	directory itemType = iota
	file
)

type item struct {
	entry    os.DirEntry
	selected bool
}

type tab struct {
	dir    string
	items  []item
	cursor int
}

type model struct {
	tabs       []tab
	currentTab int
	mode       mode
	width      int
	height     int
}

func newTab(dir string) tab {
	result := tab{dir: dir}
	return result
}

func initialModel(path string) model {
	return model{
		tabs: []tab{
			newTab(path),
		},
		currentTab: 0,
		mode:       normal,
	}
}

type errorMsg struct {
	err error
}

type readDirMsg struct {
	entries []os.DirEntry
	tab     int
}

func (m model) readDir(dir string, tab int) tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(dir)

		if err != nil {
			return errorMsg{err}
		}

		sort.Slice(entries, func(i, j int) bool {
			if entries[i].IsDir() == entries[j].IsDir() {
				return entries[i].Name() < entries[j].Name()
			}
			return entries[i].IsDir()
		})

		return readDirMsg{entries: entries, tab: tab}
	}
}

func (m model) Init() tea.Cmd {
	return m.readDir(m.tabs[0].dir, 0)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case readDirMsg:
		m.tabs[msg.tab].items = nil
		for _, entry := range msg.entries {
			m.tabs[msg.tab].items = append(m.tabs[msg.tab].items, item{entry: entry})
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			if m.mode == normal {
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	var s strings.Builder

	current_tab := &m.tabs[m.currentTab]
	s.WriteString(current_tab.dir)
	s.WriteRune('\n')

	for i, item := range m.tabs[m.currentTab].items {
		if i+1 > m.height-3 {
			break
		}

		if i == current_tab.cursor {
			s.WriteString(" > ")
		} else {
			s.WriteString("   ")
		}

		var symlinkPath string
		info, _ := item.entry.Info()
		isSymlink := info.Mode()&os.ModeSymlink != 0
		size := strings.Replace(humanize.Bytes(uint64(info.Size())), " ", "", 1) //nolint:gosec
		name := item.entry.Name()

		if isSymlink {
			symlinkPath, _ = filepath.EvalSymlinks(filepath.Join(current_tab.dir, name))
		}

		s.WriteString(name)
		s.WriteRune(' ')
		if isSymlink {
			s.WriteString("-> ")
			s.WriteString(symlinkPath)
			s.WriteRune(' ')
		}
		s.WriteString(size)
		s.WriteRune('\n')
	}

	return s.String()
}

func main() {
	p := tea.NewProgram(initialModel("C:\\"), tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		log.Fatalf("failed to launch the program: %s", err)
	}
}
