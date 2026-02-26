package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	// Set by build flags
	Version   = "dev"
	GitCommit = ""
	BuildTime = ""
)

func (m model) Init() tea.Cmd {
	return m.readDir(0, m.tabs[0].dir)
}

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.BoolVar(&showVersion, "v", false, "show version (shorthand)")
	tempFileFlag := flag.String("tf", "output.tmp", "temp file for output")
	outputFlag := flag.Bool("o", false, "enable temp file output")
	flag.Parse()
	if showVersion {
		fmt.Printf("mc %s (GitCommit: %s) Build Time: %s\n", Version, GitCommit, BuildTime)
		return
	}
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

	p := tea.NewProgram(
		initialModel(dirs),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion())

	m, err := p.Run()
	if err != nil {
		log.Fatalf("failed to launch the program: %s", err)
	}

	finalModel := m.(*model)

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
