package plugin

import (
	"context"
	"io"
)

type Task struct {
	Args   []string
	Stdout io.Writer
	Stderr io.Writer
	TaskID string
}

type Result struct {
	PluginName string
	TaskID     string
	Stdout     []byte
	Stderr     []byte
	ExitCode   int
	Err        error
}

type Plugin interface {
	Name() string
	Run(ctx context.Context, task Task) Result
}
