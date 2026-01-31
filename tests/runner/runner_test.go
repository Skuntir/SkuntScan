package runner_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"skuntir.com/SkuntScan/internal/config"
	"skuntir.com/SkuntScan/internal/plugin"
	"skuntir.com/SkuntScan/internal/runner"
)

type writerNilCheckPlugin struct {
	name string
}

func (p *writerNilCheckPlugin) Name() string { return p.name }

func (p *writerNilCheckPlugin) Run(ctx context.Context, task plugin.Task) plugin.Result {
	if task.Stdout == nil || task.Stderr == nil {
		return plugin.Result{
			PluginName: p.name,
			TaskID:     p.name + "-writers-nil",
			ExitCode:   1,
			Err:        errors.New("expected task writers to be non-nil for raw streaming"),
		}
	}
	if _, err := io.WriteString(task.Stdout, "stdout-test\n"); err != nil {
		return plugin.Result{PluginName: p.name, TaskID: p.name + "-stdout-write-fail", ExitCode: 1, Err: err}
	}
	if _, err := io.WriteString(task.Stderr, "stderr-test\n"); err != nil {
		return plugin.Result{PluginName: p.name, TaskID: p.name + "-stderr-write-fail", ExitCode: 1, Err: err}
	}
	return plugin.Result{
		PluginName: p.name,
		TaskID:     p.name + "-ok",
		ExitCode:   0,
		Stdout:     []byte("ok\n"),
	}
}

type targetsFileCheckPlugin struct {
	name string
}

func (p *targetsFileCheckPlugin) Name() string { return p.name }

func (p *targetsFileCheckPlugin) Run(ctx context.Context, task plugin.Task) plugin.Result {
	if len(task.Args) != 1 {
		return plugin.Result{PluginName: p.name, TaskID: p.name + "-bad-args", ExitCode: 1, Err: errors.New("expected exactly one arg")}
	}
	path := task.Args[0]
	if !filepath.IsAbs(path) {
		return plugin.Result{PluginName: p.name, TaskID: p.name + "-not-abs", ExitCode: 1, Err: errors.New("targets file arg not absolute")}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return plugin.Result{PluginName: p.name, TaskID: p.name + "-read-fail", ExitCode: 1, Err: err}
	}
	if !bytes.Contains(b, []byte("example.com")) {
		return plugin.Result{PluginName: p.name, TaskID: p.name + "-missing-target", ExitCode: 1, Err: errors.New("expected targets file to contain example.com")}
	}
	return plugin.Result{PluginName: p.name, TaskID: p.name + "-ok", ExitCode: 0, Stdout: []byte("ok\n")}
}

func TestRunner_NonVerbose_PassesWritersForRawStreaming(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := config.Config{
		OutputDir:      tmp,
		Concurrency:    1,
		ToolTimeoutSec: 30,
		Debug:          false,
		Verbose:        false,
		FailFast:       true,
		Plugins: []config.PluginConfig{
			{Name: "check", Binary: "/bin/true", Flags: []string{}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}

	reg := plugin.NewRegistry()
	reg.Register(&writerNilCheckPlugin{name: "check"})

	r := runner.New(&cfg, reg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := r.Run(ctx, "example.com"); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestRunner_MissingTargetsFile_ReturnsError(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := config.Config{
		OutputDir:      tmp,
		Concurrency:    1,
		ToolTimeoutSec: 30,
		Debug:          false,
		Verbose:        false,
		FailFast:       true,
		Plugins: []config.PluginConfig{
			{Name: "noop", Binary: "/bin/true", Flags: []string{}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}

	reg := plugin.NewRegistry()
	reg.Register(&writerNilCheckPlugin{name: "noop"})

	r := runner.New(&cfg, reg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := r.Run(ctx, "/this/does/not/exist/targets.txt")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "targets file not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunner_PassesNormalizedTargetsFileToPlugin(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	in := filepath.Join(tmp, "in.txt")
	if err := os.WriteFile(in, []byte("example.com\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := config.Config{
		OutputDir:      tmp,
		Concurrency:    1,
		ToolTimeoutSec: 30,
		Debug:          false,
		Verbose:        false,
		FailFast:       true,
		Plugins: []config.PluginConfig{
			{Name: "checktargets", Binary: "/bin/true", Flags: []string{"{{targets_file}}"}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}

	reg := plugin.NewRegistry()
	reg.Register(&targetsFileCheckPlugin{name: "checktargets"})

	r := runner.New(&cfg, reg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := r.Run(ctx, in); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestRunner_WritesRawOutputs(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}

	tmp := t.TempDir()
	bin := "/bin/sh"
	args := []string{"-c", "echo runner-ok"}

	cfg := config.Config{
		OutputDir:      tmp,
		Concurrency:    2,
		ToolTimeoutSec: 30,
		Debug:          false,
		Verbose:        false,
		FailFast:       true,
		Plugins: []config.PluginConfig{
			{Name: "echo", Binary: bin, Flags: args},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}

	reg := plugin.NewRegistry()
	reg.Register(plugin.NewShellPlugin("echo", bin))

	r := runner.New(&cfg, reg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := r.Run(ctx, "example.com"); err != nil {
		t.Fatalf("run error: %v", err)
	}

	rawDir := filepath.Join(tmp, "example_com", "raw", "echo")
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		t.Fatalf("expected raw output dir, got error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected raw output files")
	}
	re := regexp.MustCompile(`^echo-\d{4}_\d{2}_\d{2}_\d{2}_\d{2}(-\d+)?\.(out|err|json)$`)
	for _, e := range entries {
		if !re.MatchString(e.Name()) {
			t.Fatalf("unexpected raw output file: %s", e.Name())
		}
	}
}

type blockingPlugin struct {
	name        string
	startedCh   chan struct{}
	releaseCh   chan struct{}
	completedCh chan struct{}
}

func (p *blockingPlugin) Name() string { return p.name }

func (p *blockingPlugin) Run(ctx context.Context, task plugin.Task) plugin.Result {
	close(p.startedCh)
	select {
	case <-p.releaseCh:
	case <-ctx.Done():
		return plugin.Result{PluginName: p.name, TaskID: p.name + "-ctx", ExitCode: 1, Err: ctx.Err()}
	}
	close(p.completedCh)
	return plugin.Result{PluginName: p.name, TaskID: p.name + "-ok", ExitCode: 0, Stdout: []byte("ok\n")}
}

func TestRunner_RunsToolsInConfigOrder_Sequentially(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := config.Config{
		OutputDir:      tmp,
		Concurrency:    16,
		ToolTimeoutSec: 30,
		Debug:          false,
		Verbose:        false,
		FailFast:       true,
		Plugins: []config.PluginConfig{
			{Name: "first", Binary: "/bin/true", Flags: []string{}},
			{Name: "second", Binary: "/bin/true", Flags: []string{}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}

	firstStarted := make(chan struct{})
	firstRelease := make(chan struct{})
	firstDone := make(chan struct{})
	secondStarted := make(chan struct{})
	secondDone := make(chan struct{})
	secondRelease := make(chan struct{})
	close(secondRelease)

	reg := plugin.NewRegistry()
	reg.Register(&blockingPlugin{name: "first", startedCh: firstStarted, releaseCh: firstRelease, completedCh: firstDone})
	reg.Register(&blockingPlugin{name: "second", startedCh: secondStarted, releaseCh: secondRelease, completedCh: secondDone})

	r := runner.New(&cfg, reg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	runDone := make(chan error, 1)
	go func() { runDone <- r.Run(ctx, "example.com") }()

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for first tool to start")
	}
	rawDir := filepath.Join(tmp, "example_com", "raw", "first")
	if _, err := os.Stat(rawDir); err != nil {
		t.Fatalf("expected raw dir to exist while tool is running, got %v", err)
	}
	outs, err := filepath.Glob(filepath.Join(rawDir, "first-*.out"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(outs) == 0 {
		t.Fatalf("expected stdout file to exist while tool is running")
	}

	select {
	case <-secondStarted:
		t.Fatalf("second tool started before first completed (expected sequential order)")
	case <-time.After(250 * time.Millisecond):
	}

	close(firstRelease)

	select {
	case <-secondStarted:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for second tool to start")
	}

	select {
	case err := <-runDone:
		if err == nil {
			return
		}
		t.Fatalf("run error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for run to finish")
	}
}

type sleepPlugin struct {
	name  string
	sleep time.Duration
}

func (p *sleepPlugin) Name() string { return p.name }

func (p *sleepPlugin) Run(ctx context.Context, task plugin.Task) plugin.Result {
	select {
	case <-time.After(p.sleep):
		return plugin.Result{PluginName: p.name, TaskID: p.name + "-ok", ExitCode: 0, Stdout: []byte("ok\n")}
	case <-ctx.Done():
		return plugin.Result{PluginName: p.name, TaskID: p.name + "-ctx", ExitCode: 1, Err: ctx.Err()}
	}
}

func TestRunner_ToolTimeoutDisabled_DoesNotCancelTool(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := config.Config{
		OutputDir:      tmp,
		Concurrency:    1,
		ToolTimeoutSec: 0,
		Debug:          false,
		Verbose:        false,
		FailFast:       true,
		Plugins: []config.PluginConfig{
			{Name: "sleep", Binary: "/bin/true", Flags: []string{}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}

	reg := plugin.NewRegistry()
	reg.Register(&sleepPlugin{name: "sleep", sleep: 200 * time.Millisecond})

	r := runner.New(&cfg, reg)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := r.Run(ctx, "example.com"); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestRunner_PluginTimeoutOverride_Disabled_DoesNotCancelTool(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	disabled := 0
	cfg := config.Config{
		OutputDir:      tmp,
		Concurrency:    1,
		ToolTimeoutSec: 1,
		Debug:          false,
		Verbose:        false,
		FailFast:       true,
		Plugins: []config.PluginConfig{
			{Name: "sleep", Binary: "/bin/true", TimeoutSec: &disabled, Flags: []string{}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}

	reg := plugin.NewRegistry()
	reg.Register(&sleepPlugin{name: "sleep", sleep: 200 * time.Millisecond})

	r := runner.New(&cfg, reg)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := r.Run(ctx, "example.com"); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

type emitTargetsPlugin struct {
	name  string
	lines []string
}

func (p *emitTargetsPlugin) Name() string { return p.name }

func (p *emitTargetsPlugin) Run(ctx context.Context, task plugin.Task) plugin.Result {
	return plugin.Result{
		PluginName: p.name,
		TaskID:     p.name + "-ok",
		ExitCode:   0,
		Stdout:     []byte(strings.Join(p.lines, "\n") + "\n"),
	}
}

type readTargetsFilePlugin struct {
	name string
}

func (p *readTargetsFilePlugin) Name() string { return p.name }

func (p *readTargetsFilePlugin) Run(ctx context.Context, task plugin.Task) plugin.Result {
	if len(task.Args) != 1 {
		return plugin.Result{PluginName: p.name, TaskID: p.name + "-bad-args", ExitCode: 1, Err: errors.New("expected targets_file arg")}
	}
	b, err := os.ReadFile(task.Args[0])
	if err != nil {
		return plugin.Result{PluginName: p.name, TaskID: p.name + "-read-fail", ExitCode: 1, Err: err}
	}
	return plugin.Result{PluginName: p.name, TaskID: p.name + "-ok", ExitCode: 0, Stdout: b}
}

func TestRunner_ProducesTargets_RewritesTargetsForNextTool(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := config.Config{
		OutputDir:      tmp,
		Concurrency:    1,
		ToolTimeoutSec: 0,
		Debug:          false,
		Verbose:        false,
		FailFast:       true,
		Plugins: []config.PluginConfig{
			{Name: "subfinder", Binary: "/bin/true", ProducesTargets: true, Flags: []string{}},
			{Name: "nuclei", Binary: "/bin/true", Flags: []string{"{{targets_file}}"}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}

	reg := plugin.NewRegistry()
	reg.Register(&emitTargetsPlugin{name: "subfinder", lines: []string{"a.example.com", "B.Example.com", "http://c.example.com/"}})
	reg.Register(&readTargetsFilePlugin{name: "nuclei"})

	r := runner.New(&cfg, reg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := r.Run(ctx, "example.com"); err != nil {
		t.Fatalf("run error: %v", err)
	}

	derived := filepath.Join(tmp, "example_com", "input", "targets.subfinder.txt")
	b, err := os.ReadFile(derived)
	if err != nil {
		t.Fatalf("expected derived targets file, got %v", err)
	}
	got := strings.TrimSpace(string(b))
	if !strings.Contains(got, "a.example.com") || !strings.Contains(got, "b.example.com") || !strings.Contains(got, "c.example.com") {
		t.Fatalf("unexpected derived targets: %q", got)
	}
}

func TestRunner_ExplicitTargetsPlaceholder_UsesProducingToolTargets(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := config.Config{
		OutputDir:      tmp,
		Concurrency:    1,
		ToolTimeoutSec: 0,
		Debug:          false,
		Verbose:        false,
		FailFast:       true,
		Plugins: []config.PluginConfig{
			{Name: "Subfinder", Binary: "/bin/true", ProducesTargets: true, Flags: []string{}},
			{Name: "Nuclei", Binary: "/bin/true", Flags: []string{"{{targets_file_subfinder}}"}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}

	reg := plugin.NewRegistry()
	reg.Register(&emitTargetsPlugin{name: "Subfinder", lines: []string{"a.example.com"}})
	reg.Register(&readTargetsFilePlugin{name: "Nuclei"})

	r := runner.New(&cfg, reg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := r.Run(ctx, "example.com"); err != nil {
		t.Fatalf("run error: %v", err)
	}

	derived := filepath.Join(tmp, "example_com", "input", "targets.subfinder.txt")
	b, err := os.ReadFile(derived)
	if err != nil {
		t.Fatalf("expected derived targets file, got %v", err)
	}
	got := strings.TrimSpace(string(b))
	if got != "a.example.com\nexample.com" {
		t.Fatalf("unexpected derived targets: %q", got)
	}
}
