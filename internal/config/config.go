package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type PluginConfig struct {
	Name           string   `yaml:"name"`
	Binary         string   `yaml:"binary"`
	Enabled        *bool    `yaml:"enabled,omitempty"`
	ProducesTargets bool    `yaml:"produces_targets,omitempty"`
	TimeoutSec     *int     `yaml:"timeout_sec,omitempty"`
	Flags          []string `yaml:"flags"`
}

type Config struct {
	OutputDir      string         `yaml:"output_dir"`
	Concurrency    int            `yaml:"concurrency"`
	ToolTimeoutSec int            `yaml:"tool_timeout_sec"`
	Debug          bool           `yaml:"debug"`
	Verbose        bool           `yaml:"verbose"`
	FailFast       bool           `yaml:"fail_fast"`
	Plugins        []PluginConfig `yaml:"plugins"`
}

func LoadYAML(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	c.OutputDir = expandUserPath(c.OutputDir)
	if c.OutputDir == "" {
		return errors.New("output_dir empty")
	}
	if c.Concurrency <= 0 {
		return errors.New("concurrency must be >= 1")
	}
	if c.ToolTimeoutSec < 0 {
		return errors.New("tool_timeout_sec must be >= 0 (0 disables timeouts)")
	}
	for i := range c.Plugins {
		p := c.Plugins[i]
		if p.Name == "" {
			return fmt.Errorf("plugins[%d].name empty", i)
		}
		if p.Binary == "" {
			return fmt.Errorf("plugins[%d].binary empty", i)
		}
		if p.TimeoutSec != nil && *p.TimeoutSec < 0 {
			return fmt.Errorf("plugins[%d].timeout_sec must be >= 0", i)
		}
		p.Binary = expandUserPath(p.Binary)
		c.Plugins[i].Binary = p.Binary
		if runtime.GOOS == "linux" && (p.Enabled == nil || *p.Enabled) {
			if !filepath.IsAbs(p.Binary) || !strings.HasPrefix(p.Binary, "/") {
				return fmt.Errorf("plugins[%d].binary must be an absolute Linux path", i)
			}
		}
	}
	if err := os.MkdirAll(c.OutputDir, 0o755); err != nil {
		return err
	}
	if abs, err := filepath.Abs(c.OutputDir); err == nil {
		c.OutputDir = abs
	}
	return nil
}

func expandUserPath(p string) string {
	if p == "" {
		return ""
	}
	p = expandEnvKeepUnknown(p)
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return p
		}
		if p == "~" {
			return home
		}
		return filepath.Join(home, p[2:])
	}
	if strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	return p
}

func expandEnvKeepUnknown(s string) string {
	return os.Expand(s, func(key string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		if key == "HOME" || key == "USERPROFILE" {
			home, err := os.UserHomeDir()
			if err == nil && home != "" {
				return home
			}
		}
		return "$" + key
	})
}
