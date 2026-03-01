package main

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type ToolConfig struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
	Type    string   `toml:"type"`
}

type Config struct {
	F3  *ToolConfig `toml:"F3"`
	F4  *ToolConfig `toml:"F4"`
	F6  *ToolConfig `toml:"F6"`
	F7  *ToolConfig `toml:"F7"`
	F8  *ToolConfig `toml:"F8"`
	F9  *ToolConfig `toml:"F9"`
	F10 *ToolConfig `toml:"F10"`
	F11 *ToolConfig `toml:"F11"`
	F12 *ToolConfig `toml:"F12"`
}

func getConfigDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		appData = home
	}
	return filepath.Join(appData, ".mc"), nil
}

func getConfigPath() (string, error) {
	dir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

func loadConfig() (*Config, error) {
	cfg := &Config{
		F3: &ToolConfig{Command: "bat", Args: []string{"--color=always", "-p", "--pager", "less -c -R -S"}, Type: "path"},
		F4: &ToolConfig{Command: "hx", Args: []string{}, Type: "path"},
		F6: &ToolConfig{Command: "explorer", Args: []string{}, Type: "dir"},
		F7: &ToolConfig{Command: "code", Args: []string{}, Type: "path"},
		F8: &ToolConfig{Command: "code", Args: []string{}, Type: "dir"},
	}

	configPath, err := getConfigPath()

	if err != nil {
		return cfg, err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil
	} else if err != nil {
		return cfg, err
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
