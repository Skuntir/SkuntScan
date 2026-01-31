package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skuntir.com/SkuntScan/internal/config"
	"skuntir.com/SkuntScan/internal/output"
	"skuntir.com/SkuntScan/internal/plugin"
	"skuntir.com/SkuntScan/internal/progress"
)

type Runner struct {
	cfg     *config.Config
	outBase string
	reg     *plugin.Registry
	sem     chan struct{}
}

func New(cfg *config.Config, reg *plugin.Registry) *Runner {
	return &Runner{
		cfg:     cfg,
		outBase: cfg.OutputDir,
		reg:     reg,
		sem:     make(chan struct{}, cfg.Concurrency),
	}
}

func (r *Runner) Run(ctx context.Context, targetsPathOrSingle string) error {
	targets, err := readTargets(targetsPathOrSingle)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return errors.New("no targets found")
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if r.cfg.FailFast {
		runCtx, cancel = context.WithCancel(ctx)
		defer cancel()
	}

	groups := groupTargetsByApex(targets)
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var outErr error
	for _, apex := range keys {
		groupTargets := groups[apex]
		groupDir := filepath.Join(r.outBase, sanitizeDirName(apex))

		groupWriter, err := output.NewFileWriter(groupDir)
		if err != nil {
			return err
		}

		inputDir := filepath.Join(groupWriter.BaseDir(), "input")
		if err := os.MkdirAll(inputDir, 0o755); err != nil {
			return err
		}
		targetsFile := filepath.Join(inputDir, "targets.txt")
		if err := os.WriteFile(targetsFile, []byte(strings.Join(groupTargets, "\n")+"\n"), 0o644); err != nil {
			return err
		}
		if normalized, err := normalizeTargetsFile(targetsFile); err == nil && len(normalized) > 0 {
			_ = os.WriteFile(targetsFile, []byte(strings.Join(normalized, "\n")+"\n"), 0o644)
		}
		currentTargetsFile := targetsFile
		if r.cfg.Verbose {
			fmt.Fprintf(os.Stdout, "%-8s  %-18s  %-12s  %-6s  dir=%s\n", time.Now().Format("15:04:05"), apex, "-", "info", groupWriter.BaseDir())
			fmt.Fprintf(os.Stdout, "%-8s  %-18s  %-12s  %-6s  targets=%s\n", time.Now().Format("15:04:05"), apex, "-", "info", targetsFile)
		}

		vars := map[string]string{
			"{{targets_file}}":      currentTargetsFile,
			"{{base_targets_file}}": targetsFile,
			"{{output_dir}}":        groupWriter.BaseDir(),
		}

		type runItem struct {
			pc     config.PluginConfig
			pl     plugin.Plugin
			rowIdx int
		}
		items := make([]runItem, 0, len(r.cfg.Plugins))
		names := make([]string, 0, len(r.cfg.Plugins))
		skipped := make([]int, 0, len(r.cfg.Plugins))
		for _, pc := range r.cfg.Plugins {
			rowIdx := len(names)
			pl, ok := r.reg.Get(pc.Name)
			names = append(names, pc.Name)
			vars[targetsFileVar(pc.Name)] = ""
			vars[stdoutFileVar(pc.Name)] = ""
			vars[stderrFileVar(pc.Name)] = ""
			if pc.Enabled != nil && !*pc.Enabled {
				skipped = append(skipped, rowIdx)
				continue
			}
			if !ok {
				return fmt.Errorf("plugin %s not registered", pc.Name)
			}
			items = append(items, runItem{pc: pc, pl: pl, rowIdx: rowIdx})
		}

		var table *progress.Table
		if !r.cfg.Verbose {
			table = progress.New(os.Stdout, apex, names)
			table.Print()
			for _, i := range skipped {
				table.MarkSkipped(i)
			}
		} else {
			fmt.Fprintf(os.Stdout, "%-8s  %-18s  %-12s  %-6s  %-6s  %-4s\n", "TIME", "APEX", "TOOL", "EVENT", "DUR", "EXIT")
			now := time.Now().Format("15:04:05")
			skipSet := make(map[int]struct{}, len(skipped))
			for _, i := range skipped {
				skipSet[i] = struct{}{}
			}
			for i, name := range names {
				if _, isSkipped := skipSet[i]; isSkipped {
					fmt.Fprintf(os.Stdout, "%-8s  %-18s  %-12s  %-6s  %-6s  %-4s\n", now, apex, name, "skipped", "0ms", "0")
					continue
				}
				fmt.Fprintf(os.Stdout, "%-8s  %-18s  %-12s  %-6s  %-6s  %-4s\n", now, apex, name, "queued", "-", "-")
			}
		}

		for _, it := range items {
			vars["{{targets_file}}"] = currentTargetsFile
			vars[targetsFileVar(it.pc.Name)] = currentTargetsFile
			taskArgs := substituteArgs(it.pc.Flags, vars)
			var (
				ctxT    context.Context
				cancelT context.CancelFunc
			)
			timeoutSec := r.cfg.ToolTimeoutSec
			if it.pc.TimeoutSec != nil {
				timeoutSec = *it.pc.TimeoutSec
			}
			if timeoutSec == 0 {
				ctxT, cancelT = context.WithCancel(runCtx)
			} else {
				ctxT, cancelT = context.WithTimeout(runCtx, time.Duration(timeoutSec)*time.Second)
			}

			start := time.Now()
			taskID := it.pc.Name + "-" + start.Format("20060102T150405.000000000")

			pluginDir := filepath.Join(groupWriter.BaseDir(), "raw", it.pc.Name)
			if err := os.MkdirAll(pluginDir, 0o755); err != nil {
				cancelT()
				return err
			}
			stdoutPath := filepath.Join(pluginDir, taskID+".out")
			stderrPath := filepath.Join(pluginDir, taskID+".err")
			stdoutF, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				cancelT()
				return err
			}
			stderrF, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				_ = stdoutF.Close()
				cancelT()
				return err
			}

			vars[stdoutFileVar(it.pc.Name)] = stdoutPath
			vars[stderrFileVar(it.pc.Name)] = stderrPath

			var stdoutW io.Writer
			var stderrW io.Writer
			var stdoutPW *progress.PrefixWriter
			var stderrPW *progress.PrefixWriter
			if r.cfg.Verbose {
				stdoutPW = progress.NewPrefixWriter(os.Stdout, progress.TimePrefix(apex, it.pc.Name, "stdout"))
				stderrPW = progress.NewPrefixWriter(os.Stdout, progress.TimePrefix(apex, it.pc.Name, "stderr"))
				stdoutW = io.MultiWriter(stdoutF, stdoutPW)
				stderrW = io.MultiWriter(stderrF, stderrPW)
				fmt.Fprintf(os.Stdout, "%-8s  %-18s  %-12s  %-6s  %-6s  %-4s\n", time.Now().Format("15:04:05"), apex, it.pc.Name, "start", "-", "-")
				if timeoutSec == 0 {
					fmt.Fprintf(os.Stdout, "%-8s  %-18s  %-12s  %-6s  disabled\n", time.Now().Format("15:04:05"), apex, it.pc.Name, "timeout")
				} else {
					fmt.Fprintf(os.Stdout, "%-8s  %-18s  %-12s  %-6s  %ds\n", time.Now().Format("15:04:05"), apex, it.pc.Name, "timeout", timeoutSec)
				}
				fmt.Fprintf(os.Stdout, "%-8s  %-18s  %-12s  %-6s  %s\n", time.Now().Format("15:04:05"), apex, it.pc.Name, "cmd", formatCommand(it.pc.Binary, redactArgs(taskArgs)))
			} else if table != nil {
				stdoutW = stdoutF
				stderrW = stderrF
				table.MarkStart(it.rowIdx)
			} else {
				stdoutW = stdoutF
				stderrW = stderrF
			}

			res := it.pl.Run(ctxT, plugin.Task{Args: taskArgs, Stdout: stdoutW, Stderr: stderrW, TaskID: taskID})
			dur := time.Since(start)

			cancelT()

			_ = stdoutF.Close()
			_ = stderrF.Close()

			if stdoutPW != nil {
				_ = stdoutPW.Flush()
			}
			if stderrPW != nil {
				_ = stderrPW.Flush()
			}

			if wErr := groupWriter.WritePluginResult(&res); wErr != nil {
				if table != nil {
					table.MarkFail(it.rowIdx, res.ExitCode, dur)
				}
				if outErr == nil {
					outErr = wErr
				} else {
					outErr = fmt.Errorf("%v; %w", outErr, wErr)
				}
				if r.cfg.FailFast && cancel != nil {
					cancel()
					if table != nil {
						table.Close()
					}
					return outErr
				}
				continue
			}

			if res.Err != nil {
				if r.cfg.Verbose {
					fmt.Fprintf(os.Stdout, "%-8s  %-18s  %-12s  %-6s  %-6s  %-4d\n", time.Now().Format("15:04:05"), apex, it.pc.Name, "fail", progress.FormatDuration(dur), res.ExitCode)
					fmt.Fprintf(os.Stdout, "%-8s  %-18s  %-12s  %-6s  %v\n", time.Now().Format("15:04:05"), apex, it.pc.Name, "error", res.Err)
				} else if table != nil {
					table.MarkFail(it.rowIdx, res.ExitCode, dur)
				}
				e := fmt.Errorf("%s failed: %w", it.pl.Name(), res.Err)
				if outErr == nil {
					outErr = e
				} else {
					outErr = fmt.Errorf("%v; %w", outErr, e)
				}
				if r.cfg.FailFast && cancel != nil {
					cancel()
					if table != nil {
						table.Close()
					}
					return outErr
				}
				continue
			}

			if r.cfg.Verbose {
				fmt.Fprintf(os.Stdout, "%-8s  %-18s  %-12s  %-6s  %-6s  %-4d\n", time.Now().Format("15:04:05"), apex, it.pc.Name, "done", progress.FormatDuration(dur), res.ExitCode)
			} else if table != nil {
				table.MarkDone(it.rowIdx, res.ExitCode, dur)
			}

			if it.pc.ProducesTargets && res.Err == nil && res.ExitCode == 0 {
				nextTargets, err := normalizeTargetsFile(stdoutPath)
				if err != nil && len(res.Stdout) > 0 {
					nextTargets, err = normalizeTargetsBytes(res.Stdout)
				}
				if err != nil {
					if r.cfg.Verbose {
						fmt.Fprintf(os.Stdout, "%-8s  %-18s  %-12s  %-6s  %v\n", time.Now().Format("15:04:05"), apex, it.pc.Name, "error", err)
					}
					if outErr == nil {
						outErr = err
					} else {
						outErr = fmt.Errorf("%v; %w", outErr, err)
					}
					if r.cfg.FailFast && cancel != nil {
						cancel()
						if table != nil {
							table.Close()
						}
						return outErr
					}
					continue
				}
				merged := mergeTargets(currentTargetsFile, nextTargets)
				derived := filepath.Join(inputDir, "targets."+sanitizeDirName(it.pc.Name)+".txt")
				if err := os.WriteFile(derived, []byte(strings.Join(merged, "\n")+"\n"), 0o644); err != nil {
					if outErr == nil {
						outErr = err
					} else {
						outErr = fmt.Errorf("%v; %w", outErr, err)
					}
					if r.cfg.FailFast && cancel != nil {
						cancel()
						if table != nil {
							table.Close()
						}
						return outErr
					}
					continue
				}
				currentTargetsFile = derived
				vars[targetsFileVar(it.pc.Name)] = currentTargetsFile
				if r.cfg.Verbose {
					fmt.Fprintf(os.Stdout, "%-8s  %-18s  %-12s  %-6s  targets=%s\n", time.Now().Format("15:04:05"), apex, it.pc.Name, "info", currentTargetsFile)
				}
			}
		}

		if table != nil {
			table.Close()
		}

		if r.cfg.FailFast && outErr != nil {
			return outErr
		}
	}

	return outErr
}

func readTargets(pathOrSingle string) ([]string, error) {
	s := strings.TrimSpace(pathOrSingle)
	if s == "" {
		return nil, errors.New("empty target")
	}

	fi, err := os.Stat(s)
	if err != nil {
		if strings.Contains(s, "://") {
			return []string{s}, nil
		}
		if looksLikeFilePath(s) {
			return nil, fmt.Errorf("targets file not found: %s", s)
		}
		return []string{s}, nil
	}
	if fi.IsDir() {
		return nil, errors.New("targets path is a directory")
	}
	f, err := os.Open(s)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func looksLikeFilePath(s string) bool {
	in := strings.TrimSpace(s)
	if in == "" {
		return false
	}
	if in == "~" || strings.HasPrefix(in, "~/") {
		return true
	}
	if strings.ContainsAny(in, `/\`) {
		return true
	}
	low := strings.ToLower(in)
	return strings.HasSuffix(low, ".txt") || strings.HasSuffix(low, ".list") || strings.HasSuffix(low, ".lst")
}

func substituteArgs(args []string, vars map[string]string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		s := a
		for k, v := range vars {
			s = strings.ReplaceAll(s, k, v)
		}
		out = append(out, s)
	}
	return out
}

func targetsFileVar(toolName string) string {
	return "{{targets_file_" + sanitizeDirName(toolName) + "}}"
}

func stdoutFileVar(toolName string) string {
	return "{{stdout_file_" + sanitizeDirName(toolName) + "}}"
}

func stderrFileVar(toolName string) string {
	return "{{stderr_file_" + sanitizeDirName(toolName) + "}}"
}

func normalizeTargetsBytes(b []byte) ([]string, error) {
	lines := strings.Split(string(b), "\n")
	seen := make(map[string]struct{}, len(lines))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, "#") {
			continue
		}
		s = strings.TrimPrefix(s, "http://")
		s = strings.TrimPrefix(s, "https://")
		s = strings.TrimSuffix(s, "/")
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, errors.New("produced targets file is empty")
	}
	return out, nil
}

func normalizeTargetsFile(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return normalizeTargetsBytes(b)
}

func mergeTargets(currentTargetsPath string, newTargets []string) []string {
	seen := make(map[string]struct{}, len(newTargets)+1024)
	out := make([]string, 0, len(newTargets)+1024)

	if currentTargetsPath != "" {
		if existing, err := normalizeTargetsFile(currentTargetsPath); err == nil {
			for _, s := range existing {
				if s == "" {
					continue
				}
				if _, ok := seen[s]; ok {
					continue
				}
				seen[s] = struct{}{}
				out = append(out, s)
			}
		}
	}

	for _, s := range newTargets {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	sort.Strings(out)
	return out
}

func groupTargetsByApex(targets []string) map[string][]string {
	out := make(map[string][]string)
	for _, t := range targets {
		host := extractHost(t)
		apex := apexDomain(host)
		out[apex] = append(out[apex], t)
	}
	return out
}

func extractHost(target string) string {
	s := strings.TrimSpace(target)
	if s == "" {
		return ""
	}
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err == nil && u.Host != "" {
			h := u.Host
			if host, _, err := net.SplitHostPort(h); err == nil {
				return strings.ToLower(host)
			}
			return strings.ToLower(h)
		}
	}

	if host, _, err := net.SplitHostPort(s); err == nil {
		return strings.ToLower(host)
	}

	return strings.ToLower(s)
}

func apexDomain(host string) string {
	h := strings.Trim(host, ".")
	if h == "" {
		return "unknown"
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.String()
	}

	parts := strings.Split(h, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return h
}

func sanitizeDirName(s string) string {
	in := strings.ToLower(strings.TrimSpace(s))
	if in == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(in))
	for _, r := range in {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '-' || r == '_':
			b.WriteByte('_')
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	out = strings.Trim(out, "_")
	if out == "" {
		return "unknown"
	}
	return out
}

func formatCommand(bin string, args []string) string {
	if len(args) == 0 {
		return bin
	}
	return bin + " " + strings.Join(args, " ")
}

func redactArgs(args []string) []string {
	out := make([]string, 0, len(args))
	redactNext := false
	for _, a := range args {
		low := strings.ToLower(a)
		if redactNext {
			out = append(out, "REDACTED")
			redactNext = false
			continue
		}
		if strings.Contains(low, "token") || strings.Contains(low, "secret") || strings.Contains(low, "apikey") || strings.Contains(low, "api-key") || strings.Contains(low, "password") || strings.Contains(low, "pass") || strings.Contains(low, "bearer") {
			if strings.Contains(a, "=") {
				parts := strings.SplitN(a, "=", 2)
				out = append(out, parts[0]+"=REDACTED")
			} else {
				out = append(out, a)
				redactNext = true
			}
			continue
		}
		out = append(out, a)
	}
	return out
}
