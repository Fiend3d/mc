package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

var (
	// Set by build flags
	Version   = "2.1.3-dev"
	GitCommit = ""
	BuildTime = ""
)

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

	finalModel := initialModel(dirs)
	if err := run(&finalModel); err != nil {
		log.Fatalf("mc: %s", err)
	}

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
