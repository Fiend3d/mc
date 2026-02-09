package main

import (
	"os"
)

// DirExists checks if a path exists and is a directory
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// PathExists checks if a path exists (file or directory)
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsFile checks if a path exists and is a file (not directory)
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func numberOfDigits(n int) int {
	if n == 0 {
		return 1
	}
	if n < 0 {
		n = -n
	}

	count := 0
	for n > 0 {
		n /= 10
		count++
	}
	return count
}
