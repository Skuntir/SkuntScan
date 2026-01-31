package plugin

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"time"
)

type ShellPlugin struct {
	name string
	bin  string
}

func NewShellPlugin(name, binary string) *ShellPlugin {
	return &ShellPlugin{name: name, bin: binary}
}

func (s *ShellPlugin) Name() string { return s.name }

func (s *ShellPlugin) Run(ctx context.Context, task Task) Result {
	cmd := exec.CommandContext(ctx, s.bin, task.Args...)
	var stdout, stderr bytes.Buffer
	if task.Stdout != nil {
		cmd.Stdout = io.MultiWriter(&stdout, task.Stdout)
	} else {
		cmd.Stdout = &stdout
	}
	if task.Stderr != nil {
		cmd.Stderr = io.MultiWriter(&stderr, task.Stderr)
	} else {
		cmd.Stderr = &stderr
	}

	start := time.Now()
	err := cmd.Run()

	taskID := task.TaskID
	if taskID == "" {
		taskID = s.name + "-" + start.Format("2006_01_02_15_04")
	}
	res := Result{
		PluginName: s.name,
		TaskID:     taskID,
		Stdout:     stdout.Bytes(),
		Stderr:     stderr.Bytes(),
		ExitCode:   0,
		Err:        nil,
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
		}
		res.Err = err
	}

	return res
}
