package config_test

import (
	"runtime"
	"testing"

	"skuntir.com/SkuntScan/internal/config"
)

func boolPtr(v bool) *bool { return &v }

func TestValidate_ExpandsHomeEnvInPluginBinary(t *testing.T) {
	t.Setenv("HOME", "/tmp/home")

	cfg := config.Config{
		OutputDir:      t.TempDir(),
		Concurrency:    1,
		ToolTimeoutSec: 0,
		Plugins: []config.PluginConfig{
			{Name: "Tool", Binary: "$HOME/bin/tool", Flags: []string{}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.Plugins[0].Binary != "/tmp/home/bin/tool" {
		t.Fatalf("expected %q, got %q", "/tmp/home/bin/tool", cfg.Plugins[0].Binary)
	}
}

func TestValidate_KeepsUnknownEnvVarsInPluginBinary(t *testing.T) {
	t.Setenv("SKUNTSCAN_TEST_UNKNOWN", "")

	cfg := config.Config{
		OutputDir:      t.TempDir(),
		Concurrency:    1,
		ToolTimeoutSec: 0,
		Plugins: []config.PluginConfig{
			{
				Name:    "Tool",
				Binary:  "$SKUNTSCAN_TEST_UNKNOWN/bin",
				Enabled: boolPtr(false),
				Flags:   []string{},
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.Plugins[0].Binary != "$SKUNTSCAN_TEST_UNKNOWN/bin" {
		t.Fatalf("expected %q, got %q", "$SKUNTSCAN_TEST_UNKNOWN/bin", cfg.Plugins[0].Binary)
	}
}

func TestValidate_HomeFallback_WindowsUserProfile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only HOME fallback behavior")
	}

	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", `C:\Users\Alice`)

	cfg := config.Config{
		OutputDir:      t.TempDir(),
		Concurrency:    1,
		ToolTimeoutSec: 0,
		Plugins: []config.PluginConfig{
			{Name: "Tool", Binary: `$HOME\bin\tool.exe`, Flags: []string{}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.Plugins[0].Binary != `C:\Users\Alice\bin\tool.exe` {
		t.Fatalf("expected %q, got %q", `C:\Users\Alice\bin\tool.exe`, cfg.Plugins[0].Binary)
	}
}
