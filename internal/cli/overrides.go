package cli

import (
	"os"
	"path/filepath"

	"skuntir.com/SkuntScan/internal/config"
)

func ResolveOutputBase(outFlag string, targetsPath string) (string, error) {
	if outFlag != "" {
		return filepath.Abs(outFlag)
	}
	if targetsPath != "" {
		if fi, err := os.Stat(targetsPath); err == nil && !fi.IsDir() {
			abs, err := filepath.Abs(targetsPath)
			if err != nil {
				return "", err
			}
			return filepath.Dir(abs), nil
		}
	}
	return os.Getwd()
}

func ApplyOutputOverride(cfg *config.Config, outFlag string, targetsPath string) error {
	if outFlag == "" {
		return nil
	}
	outBase, err := ResolveOutputBase(outFlag, targetsPath)
	if err != nil {
		return err
	}
	cfg.OutputDir = outBase
	return nil
}

