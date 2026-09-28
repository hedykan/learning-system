package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hedykan/learning-system/internal/fsutil"
	"gopkg.in/yaml.v3"
)

type Model struct {
	Provider string `yaml:"provider" json:"provider"`
	Name     string `yaml:"model" json:"model"`
}

type Git struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

type Curriculum struct {
	Active string `yaml:"active" json:"active"`
}

type Config struct {
	Version    int        `yaml:"version" json:"version"`
	Language   string     `yaml:"language,omitempty" json:"language,omitempty"` // interface language; empty means zh
	Model      Model      `yaml:"model" json:"model"`
	Git        Git        `yaml:"git" json:"git"`
	Curriculum Curriculum `yaml:"curriculum" json:"curriculum"`
}

func Default() Config {
	return Config{
		Version: 1,
		Model:   Model{Provider: "builtin", Name: "conservative-v0.1"},
		Git:     Git{Enabled: true},
	}
}

func Load(root string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(root, ".learning", "config.yaml"))
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Version != 1 {
		return Config{}, fmt.Errorf("unsupported config version %d", cfg.Version)
	}
	return cfg, nil
}

func Save(root string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return fsutil.WriteFileAtomic(filepath.Join(root, ".learning", "config.yaml"), data, 0o644)
}
