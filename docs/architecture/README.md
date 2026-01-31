# Architecture

SkuntScan is a Go CLI orchestrator. It runs external recon/scanning tools as subprocesses, applies consistent execution semantics (timeouts, cancellation, deterministic ordering), and persists raw outputs in a deterministic directory layout for later analysis.

## Design Goals

- Treat external tools as pluggable workers, not re-implementations.
- Keep execution predictable and observable (raw stdout/stderr captured per run).
- Make runs reproducible and debuggable (stable task IDs, canonical output layout).
- Keep the core small and testable (runner tests do not require third-party tools).

## High-Level Data Flow

1. CLI loads config and validates invariants.
2. Runner normalizes input targets to an on-disk file.
3. Runner executes configured plugins in YAML order (top to bottom), with optional per-tool timeouts.
4. Output writer persists raw artifacts for each plugin invocation.

## Package Responsibilities

- `cmd/SkuntScan`
  - CLI entrypoint and runtime wiring.
  - Loads config YAML (optional; defaults to `~/.config/SkuntScan/conf/default.yaml`), applies CLI overrides, installs signal cancellation, and launches the runner.

- `internal/config`
  - YAML schema and config validation.
  - Ensures required fields exist and basic constraints hold (output dir writable, timeouts non-negative).

- `internal/plugin`
  - Plugin interface (unit of execution) and registry.
  - Shell plugin implementation that wraps `os/exec` and returns structured results.

- `internal/runner`
  - Orchestration: target normalization, variable substitution, target chaining, timeout enforcement, and cancellation semantics.
  - Runs each enabled plugin entry from config as an independent task.

- `internal/progress`
  - Console UI for run progress (boxed table in non-verbose mode, event log + streamed tool output in verbose mode).

- `internal/output`
  - Persists raw artifacts for each plugin run:
    - `<output_dir>/<apex_dir>/raw/<plugin>/<task_id>.out` (stdout)
    - `<output_dir>/<apex_dir>/raw/<plugin>/<task_id>.err` (stderr)
    - `<output_dir>/<apex_dir>/raw/<plugin>/<task_id>.json` (metadata including exit code + file names)

## Component Interfaces

### Plugin Interface

At runtime, the runner only depends on a minimal plugin interface:

- Input: `Task` (arguments to run)
- Output: `Result` (stdout/stderr/exit code/error)

This is intentionally small so you can later add:

- Native plugins (Go implementations) without subprocesses
- Tool-specific parsers that emit structured artifacts (JSON/SARIF/etc.)

### Output Writer

The writer is responsible for:

- Creating directories as needed
- Writing stdout and metadata atomically enough for typical local usage
- Keeping filenames deterministic and collision-resistant

## Execution Model

- Ordering: tools execute in config order (top to bottom) for each apex group.
- Target chaining: tools can mark `produces_targets: true` to normalize and merge stdout into the current targets list for subsequent tools.
- Timeouts: each plugin run is wrapped in `context.WithTimeout` unless `tool_timeout_sec` is `0`.
- Cancellation: the CLI cancels the root context on Ctrl+C and SIGTERM.
- Fail-fast: if enabled, the first error cancels the shared context, allowing in-flight tasks to stop quickly.

## Extending the System

SkuntScan is designed to evolve from “run tools and capture raw outputs” into a multi-stage pipeline:

- Stage outputs written to canonical files (e.g., assets list, resolved list, alive URLs).
- Later stages consume those canonical files via additional variables (e.g., `{{alive_file}}`).
- Optional: define explicit stage ordering and dependencies (DAG) rather than “run all enabled plugins”.

## Files to Read First

- [main.go](../../cmd/SkuntScan/main.go)
- [runner.go](../../internal/runner/runner.go)
- [config.go](../../internal/config/config.go)

