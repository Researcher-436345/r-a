package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type Settings map[string]string

// Python evaluates the shared variables and their env overrides once at startup.
func ReadSettings() Settings {
	directory, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	var path string
	for {
		path = filepath.Join(directory, "settings.py")
		if _, err := os.Stat(path); err == nil {
			break
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			panic("settings.py not found: run from the project directory")
		}
		directory = parent
	}
	raw, err := exec.Command("python3", "-B", path, "--export").Output()
	if err != nil {
		panic(fmt.Errorf("evaluate settings.py with python3: %w", err))
	}
	var settings Settings
	if err := json.Unmarshal(raw, &settings); err != nil {
		panic(fmt.Errorf("decode settings.py output: %w", err))
	}
	return settings
}

func (s Settings) Value(name string) string {
	value, exists := s[name]
	if !exists {
		panic("unknown setting: " + name)
	}
	return value
}

func (s Settings) PositiveInt(name string) int {
	value, err := strconv.Atoi(s.Value(name))
	if err != nil || value <= 0 {
		panic(name + " must be a positive integer")
	}
	return value
}

func (s Settings) Bool(name string) bool {
	value, err := strconv.ParseBool(s.Value(name))
	if err != nil {
		panic(name + " must be true or false")
	}
	return value
}
