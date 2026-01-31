package cmd_test

import (
	"path/filepath"
	"testing"

	"skuntir.com/SkuntScan/internal/cli"
	"skuntir.com/SkuntScan/internal/config"
)

func TestApplyOutputOverride_DoesNotOverrideWhenOutFlagEmpty(t *testing.T) {
	cfg := config.Config{OutputDir: "/home/xor/expected"}
	if err := cli.ApplyOutputOverride(&cfg, "", "/tmp/targets.txt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OutputDir != "/home/xor/expected" {
		t.Fatalf("expected output dir to remain unchanged, got %q", cfg.OutputDir)
	}
}

func TestApplyOutputOverride_OverridesWhenOutFlagSet(t *testing.T) {
	cfg := config.Config{OutputDir: "ignored"}
	want, err := filepath.Abs("outdir")
	if err != nil {
		t.Fatalf("failed to build expected abs path: %v", err)
	}
	if err := cli.ApplyOutputOverride(&cfg, "outdir", "/tmp/targets.txt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OutputDir != want {
		t.Fatalf("expected output dir %q, got %q", want, cfg.OutputDir)
	}
}

