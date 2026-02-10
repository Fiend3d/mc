package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Init() tea.Cmd {
	return m.readDir(m.tabs[0].dir)
}

func main() {
	tempFileFlag := flag.String("tf", "output.tmp", "temp file for output")
	outputFlag := flag.Bool("o", false, "enable temp file output")
	flag.Parse()
	dirs := flag.Args()
	tempFile := *tempFileFlag
	output := *outputFlag

	if len(dirs) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("error: %s", err)
		}
		dirs = []string{wd}
	}

	p := tea.NewProgram(initialModel(dirs), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		log.Fatalf("failed to launch the program: %s", err)
	}

	finalModel := m.(model)

	if output {
		if finalModel.result != "" {
			err := os.WriteFile(tempFile, []byte(finalModel.result), 0644)
			if err != nil {
				log.Fatalf("error: %s\n", err)
			}
		}
	} else {
		fmt.Println(finalModel.result)
	}
}
