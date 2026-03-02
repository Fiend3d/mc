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
		F3: &ToolConfig{Command: "bat", Args: []string{"--color=always", "-p", "--pager", "less -c -R -S"}, Type: "path"},
		F4: &ToolConfig{Command: "hx", Args: []string{}, Type: "path"},
		F6: &ToolConfig{Command: "explorer", Args: []string{}, Type: "dir"},
		F7: &ToolConfig{Command: "code", Args: []string{}, Type: "path"},
		F8: &ToolConfig{Command: "code", Args: []string{}, Type: "dir"},
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
	err := os.MkdirAll(getConfigDir(), 0755)
	if err != nil {
		return err
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
