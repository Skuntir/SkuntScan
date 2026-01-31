package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"skuntir.com/SkuntScan/internal/branding"
	"skuntir.com/SkuntScan/internal/cli"
	"skuntir.com/SkuntScan/internal/config"
	"skuntir.com/SkuntScan/internal/plugin"
	"skuntir.com/SkuntScan/internal/runner"
)

//go:embed default/default.yaml
var embeddedDefaultConfig []byte

func main() {
	cfgPath := flag.String("config", "", "path to config yaml (optional)")
	targetsPath := flag.String("targets", "", "file with newline separated targets or single domain")
	urlTarget := flag.String("u", "", "single URL or host target (bypasses -targets)")
	out := flag.String("out", "", "output directory (overrides config)")
	concurrency := flag.Int("concurrency", 0, "max concurrent plugin runs (overrides config)")
	timeout := flag.Int("tool-timeout", -1, "per-tool timeout in seconds; 0 disables (overrides config)")
	debugFlag := flag.Bool("debug", false, "enable debug logging (overrides config)")
	verboseFlag := flag.Bool("verbose", false, "print verbose runtime logs (streams tool output)")
	verboseShort := flag.Bool("v", false, "alias for -verbose")
	failFastFlag := flag.Bool("fail-fast", false, "stop on first plugin error (overrides config)")
	regenDefault := flag.Bool("regen-default-config", false, "overwrite the default config file and exit")
	regenDefaultAlias := flag.Bool("regen-config", false, "alias for -regen-default-config")
	flag.Parse()

	if *regenDefault || *regenDefaultAlias {
		cfgFile := *cfgPath
		if cfgFile == "" {
			p, err := defaultConfigPath()
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to resolve default config path: %v\n", err)
				os.Exit(2)
			}
			cfgFile = p
		}
		if err := ensureConfigFile(cfgFile, true); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write default config: %v\n", err)
			os.Exit(2)
		}
		fmt.Fprintln(os.Stdout, cfgFile)
		return
	}

	if runtime.GOOS != "linux" {
		fmt.Fprintln(os.Stderr, "SkuntScan currently supports Linux only.")
		os.Exit(2)
	}

	if (*targetsPath == "" && *urlTarget == "") || (*targetsPath != "" && *urlTarget != "") {
		fmt.Fprintln(os.Stderr, "exactly one of -targets or -u is required")
		os.Exit(2)
	}

	cfgFile := *cfgPath
	if cfgFile == "" {
		p, err := defaultConfigPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to resolve default config path: %v\n", err)
			os.Exit(2)
		}
		cfgFile = p
		if err := ensureConfigFile(cfgFile, false); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create default config: %v\n", err)
			os.Exit(2)
		}
	}

	cfg, err := config.LoadYAML(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read config: %v\n", err)
		os.Exit(2)
	}

	if err := cli.ApplyOutputOverride(&cfg, *out, *targetsPath); err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve output directory: %v\n", err)
		os.Exit(2)
	}
	if *concurrency > 0 {
		cfg.Concurrency = *concurrency
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = runtime.NumCPU() * 2
	}
	if *timeout >= 0 {
		cfg.ToolTimeoutSec = *timeout
	}
	if *debugFlag {
		cfg.Debug = true
	}
	if *verboseFlag || *verboseShort {
		cfg.Verbose = true
	}
	if *failFastFlag {
		cfg.FailFast = true
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(2)
	}

	if cfg.Verbose {
		fmt.Print(branding.Banner())
	}

	reg := plugin.NewRegistry()
	for _, p := range cfg.Plugins {
		reg.Register(plugin.NewShellPlugin(p.Name, p.Binary))
	}

	r := runner.New(&cfg, reg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()

	targetArg := *targetsPath
	if *urlTarget != "" {
		targetArg = *urlTarget
	}

	if err := r.Run(ctx, targetArg); err != nil {
		fmt.Fprintf(os.Stderr, "run failed: %v\n", err)
		os.Exit(1)
	}

	if cfg.Verbose {
		fmt.Println("[+] Done:", cfg.OutputDir)
	}
}

func defaultConfigPath() (string, error) {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "SkuntScan", "conf", "default.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "SkuntScan", "conf", "default.yaml"), nil
}

func ensureConfigFile(path string, overwrite bool) error {
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, embeddedDefaultConfig, 0o644)
}
