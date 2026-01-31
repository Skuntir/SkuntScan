package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegenDefaultConfigFlag_OverwritesConfigFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "default.yaml")
	if err := os.WriteFile(cfgPath, []byte("OLD"), 0o644); err != nil {
		t.Fatalf("seed write error: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/SkuntScan", "-regen-default-config", "-config", cfgPath)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run error: %v\noutput:\n%s", err, string(out))
	}

	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(b) == "OLD" {
		t.Fatalf("expected config to be overwritten")
	}
	if !strings.Contains(string(b), "plugins:") {
		t.Fatalf("expected regenerated config to contain plugins:, got:\n%s", string(b))
	}
}

