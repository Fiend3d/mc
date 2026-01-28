package main

import (
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
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

type Item struct {
	path     string
	itemType itemType
	selected bool
}

type tab struct {
	dir    string
	items  []Item
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

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("failed to read path: %s", err)
	}

	for _, entry := range entries {
		item := Item{path: entry.Name()}
		if entry.IsDir() {
			item.itemType = directory
		} else {
			item.itemType = file
		}
		result.items = append(result.items, item)
	}

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

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
	ui := m.tabs[m.currentTab].dir

	return ui
}

func main() {
	p := tea.NewProgram(initialModel("C:\\"), tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		log.Fatalf("failed to launch the program: %s", err)
	}
}
