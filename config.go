package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type ToolConfig struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
	Type    string   `toml:"type"`
}

type Config struct {
	Theme string      `toml:"theme"`
	F2    *ToolConfig `toml:"F2"`
	F3    *ToolConfig `toml:"F3"`
	F4    *ToolConfig `toml:"F4"`
	F6    *ToolConfig `toml:"F6"`
	F7    *ToolConfig `toml:"F7"`
	F8    *ToolConfig `toml:"F8"`
	F9    *ToolConfig `toml:"F9"`
	F10   *ToolConfig `toml:"F10"`
	F11   *ToolConfig `toml:"F11"`
	F12   *ToolConfig `toml:"F12"`
}

func getConfigDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		appData = home
	}
	return filepath.Join(appData, "mc")
}

func getConfigPath() string {
	dir := getConfigDir()
	return filepath.Join(dir, "config.toml")
}

func loadConfig() (*Config, error) {
	cfg := &Config{
		Theme: "dracula",
		F2:    &ToolConfig{Command: "deps", Args: []string{}, Type: "path"},
		F3:    &ToolConfig{Command: "bat", Args: []string{"--color=always", "-p", "--pager", "less -c -R -S"}, Type: "path"},
		F4:    &ToolConfig{Command: "hx", Args: []string{}, Type: "path"},
		F6:    &ToolConfig{Command: "explorer", Args: []string{}, Type: "dir"},
		F7:    &ToolConfig{Command: "code", Args: []string{}, Type: "path"},
		F8:    &ToolConfig{Command: "code", Args: []string{}, Type: "dir"},
	}

	configPath := getConfigPath()

	if !pathExists(configPath) {
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, err
	}

	_, err = toml.Decode(string(data), cfg)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

func saveConfig(cfg *Config) error {
	dir := getConfigDir()
	if !dirExists(dir) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}

	f, err := os.Create(getConfigPath())
	if err != nil {
		return err
	}
	defer f.Close()

	err = toml.NewEncoder(f).Encode(cfg)
	if err != nil {
		return err
	}

	return nil
}

func getBookmarksPath() string {
	dir := getConfigDir()
	return filepath.Join(dir, "bookmarks.list")
}

const noBookmarks = "--- NONE ---"

func loadBookmarks() ([]string, error) {
	path := getBookmarksPath()
	if !pathExists(path) {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if string(data) == noBookmarks {
		return nil, nil
	}
	return strings.Split(string(data), "\n"), nil
}

func saveBookmarks(bookmarks []string) error {
	dir := getConfigDir()
	if !dirExists(dir) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}
	path := getBookmarksPath()
	var data string
	if len(bookmarks) == 0 {
		data = noBookmarks
	} else {
		data = strings.Join(bookmarks, "\n")
	}
	return os.WriteFile(path, []byte(data), 0o644)
}

const SHELL = "powershell"

func getShellHistoryPath() string {
	dir := getConfigDir()
	return filepath.Join(dir, "shell.list")
}

func loadShellHistory() ([]string, error) {
	path := getShellHistoryPath()
	if !pathExists(path) {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

func saveShellHistory(history []string, cmd string) error {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}

	if len(history) > 0 && history[0] == cmd {
		return nil
	}

	dir := getConfigDir()
	if !dirExists(dir) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	path := getShellHistoryPath()

	newHistory := make([]string, 0, 51)
	newHistory = append(newHistory, cmd)

	for _, h := range history {
		if h != cmd && len(newHistory) < 50 {
			newHistory = append(newHistory, h)
		}
	}

	data := strings.Join(newHistory, "\n")
	return os.WriteFile(path, []byte(data), 0o644)
}
