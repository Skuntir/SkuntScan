package plugin_test

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"skuntir.com/SkuntScan/internal/plugin"
)

func TestShellPlugin_Run_Echo(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}

	bin := "/bin/sh"
	args := []string{"-c", "echo hello"}

	p := plugin.NewShellPlugin("echo", bin)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := p.Run(ctx, plugin.Task{Args: args})
	if res.Err != nil {
		t.Fatalf("expected no error, got %v (stderr=%q)", res.Err, string(res.Stderr))
	}
	if !strings.Contains(string(res.Stdout), "hello") {
		t.Fatalf("expected stdout to contain hello, got %q", string(res.Stdout))
	}
}

func TestShellPlugin_Run_StreamsStdoutStderrWhenWritersProvided(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}

	bin := "/bin/sh"
	args := []string{"-c", "echo out; echo err 1>&2"}

	p := plugin.NewShellPlugin("echo", bin)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var live bytes.Buffer
	res := p.Run(ctx, plugin.Task{Args: args, Stdout: &live, Stderr: &live})
	if res.Err != nil {
		t.Fatalf("expected no error, got %v (stderr=%q)", res.Err, string(res.Stderr))
	}
	if !strings.Contains(live.String(), "out") || !strings.Contains(live.String(), "err") {
		t.Fatalf("expected live stream to contain stdout+stderr, got %q", live.String())
	}
	if !strings.Contains(string(res.Stdout), "out") {
		t.Fatalf("expected stdout to contain out, got %q", string(res.Stdout))
	}
	if !strings.Contains(string(res.Stderr), "err") {
		t.Fatalf("expected stderr to contain err, got %q", string(res.Stderr))
	}
}
