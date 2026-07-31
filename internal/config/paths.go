package config

import (
	"os"
	"path/filepath"
)

// Root 返回 ~/.wtc 目录（不保证存在）
func Root() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".wtc"), nil
}

// ConfigFile 返回 ~/.wtc/config.yml
func ConfigFile() (string, error) {
	r, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "config.yml"), nil
}

// EnvironmentsDir 返回 ~/.wtc/environments
func EnvironmentsDir() (string, error) {
	r, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "environments"), nil
}
