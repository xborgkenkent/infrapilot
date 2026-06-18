package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds InfraPilot runtime settings.
type Config struct {
	AWSRegion        string `json:"aws_region"`
	Kubeconfig       string `json:"kubeconfig"`
	TerraformState   string `json:"terraform_state"`
	SQLitePath       string `json:"sqlite_path"`
	ReadOnly         bool   `json:"read_only"`
	DefaultAWSRegion string `json:"-"`
}

// Default returns configuration with sensible defaults.
func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		AWSRegion:        envOr("INFRAPILOT_AWS_REGION", "us-east-1"),
		Kubeconfig:       envOr("KUBECONFIG", filepath.Join(home, ".kube", "config")),
		TerraformState:   envOr("INFRAPILOT_TERRAFORM_STATE", ""),
		SQLitePath:       envOr("INFRAPILOT_SQLITE_PATH", filepath.Join(home, ".infrapilot", "metadata.db")),
		ReadOnly:         true,
		DefaultAWSRegion: "us-east-1",
	}
}

// Load reads optional JSON config from path. Missing file is not an error.
func Load(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.AWSRegion == "" {
		cfg.AWSRegion = cfg.DefaultAWSRegion
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
