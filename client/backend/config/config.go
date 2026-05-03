package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"zcopy-client-backend/models"
)

func LoadConfig() (models.AppConfig, error) {
	configPath := resolveConfigPath()
	buf, err := os.ReadFile(configPath)
	if err != nil {
		return models.AppConfig{}, fmt.Errorf("read config %s: %w", configPath, err)
	}
	var cfg models.AppConfig
	if err := yaml.Unmarshal(buf, &cfg); err != nil {
		return models.AppConfig{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8090"
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = "debug"
	}
	if cfg.FileServer.BaseURL == "" {
		cfg.FileServer.BaseURL = "http://localhost:8890/api/v1"
	}
	if cfg.Storage.DataDir == "" {
		cfg.Storage.DataDir = "./data"
	}
	if envDataDir := strings.TrimSpace(os.Getenv("ZCOPY_CLIENT_DATA_DIR")); envDataDir != "" {
		cfg.Storage.DataDir = envDataDir
	}
	return cfg, nil
}

func resolveConfigPath() string {
	if envPath := strings.TrimSpace(os.Getenv("ZCOPY_CLIENT_CONFIG")); envPath != "" {
		return envPath
	}
	candidates := []string{
		filepath.Join("config", "config.yaml"),
		filepath.Join(".", "config", "config.yaml"),
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates, filepath.Join(exeDir, "config", "config.yaml"))
		if parent := filepath.Dir(exeDir); parent != exeDir {
			candidates = append(candidates, filepath.Join(parent, "config", "config.yaml"))
		}
	}
	if lookedPath, err := exec.LookPath("zcopy-client-backend.exe"); err == nil {
		binDir := filepath.Dir(lookedPath)
		candidates = append(candidates, filepath.Join(binDir, "config", "config.yaml"))
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join("config", "config.yaml")
}
