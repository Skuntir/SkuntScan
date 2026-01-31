package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"skuntir.com/SkuntScan/internal/plugin"
)

type Writer struct {
	base string
}

func NewFileWriter(base string) (*Writer, error) {
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, err
	}
	return &Writer{base: base}, nil
}

func (w *Writer) BaseDir() string { return w.base }

func (w *Writer) WritePluginResult(res *plugin.Result) error {
	pluginDir := filepath.Join(w.base, "raw", res.PluginName)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return err
	}

	stdoutPath := filepath.Join(pluginDir, res.TaskID+".out")
	if err := os.WriteFile(stdoutPath, res.Stdout, 0o644); err != nil {
		return err
	}
	stderrPath := filepath.Join(pluginDir, res.TaskID+".err")
	if err := os.WriteFile(stderrPath, res.Stderr, 0o644); err != nil {
		return err
	}

	meta := map[string]interface{}{
		"plugin":   res.PluginName,
		"task_id":  res.TaskID,
		"exit":     res.ExitCode,
		"stdout":   filepath.Base(stdoutPath),
		"stderr":   filepath.Base(stderrPath),
		"hasError": res.Err != nil,
	}
	b, _ := json.MarshalIndent(meta, "", "  ")

	metaPath := filepath.Join(pluginDir, fmt.Sprintf("%s.json", res.TaskID))
	if err := os.WriteFile(metaPath, b, 0o644); err != nil {
		return err
	}

	return nil
}

