# Semantics

This document defines what SkuntScan guarantees at runtime. These semantics are intentionally strict so results are easy to reason about.

## Inputs

### Targets

- The `-targets` argument accepts either:
  - A path to a newline-separated file of targets, or
  - A single target string (domain, subdomain, URL, IP), if the path does not exist.
- Empty lines are ignored.
- Targets are normalized for stability (trimmed, lowercased, schemes removed when present).

### Output Base Directory

- If `-out` is set, outputs are written there.
- If `-targets` points to a file, outputs are written next to that file by default.
- If a single target is provided (or `-u` mode), outputs are written to the current working directory by default.

### Configuration

- Config is loaded from YAML (`-config /path/to/config.yaml`).
- CLI flags override config fields when provided.
- Each plugin in the config becomes an execution task if it is enabled.

## Normalization

To make plugin commands deterministic and avoid ambiguity between “single target” and “file of targets”, SkuntScan always writes normalized targets files.

### Apex Grouping

Targets are grouped by apex domain (a simple last-two-label heuristic):

- `test.example.com` → `example.com`
- `other.domain.com` → `domain.com`

Each group gets its own output folder named by replacing dots with underscores:

- `example.com` → `example_com`

### Targets File

For each apex group, SkuntScan writes:

- `<output_dir>/<apex_dir>/input/targets.txt`
- Content is a normalized, de-duplicated list (sorted, one per line, trailing newline).

## Plugin Execution

### Execution Unit

- A plugin run is defined as executing one configured plugin entry with its `flags`.
- Each run produces a `Result` containing stdout, stderr, exit code, and an error value if execution failed.

### Variable Substitution

Before execution, SkuntScan substitutes template variables inside each argument string:

- `{{targets_file}}` → current targets file for this apex group (may be rewritten by producing tools)
- `{{base_targets_file}}` → original `<output_dir>/<apex_dir>/input/targets.txt`
- `{{targets_file_<tool>}}` → explicit targets file for a tool name (example: `{{targets_file_subfinder}}`)
- `{{stdout_file_<tool>}}` → stdout file path for the most recent run of a tool (example: `{{stdout_file_katana}}`)
- `{{stderr_file_<tool>}}` → stderr file path for the most recent run of a tool (example: `{{stderr_file_katana}}`)
- `{{output_dir}}` → absolute path to the current apex group output directory

Notes:

- Substitution is purely textual; it does not understand quoting rules.
- If you need spaces or special shell quoting, prefer passing arguments as separate YAML items.

### Ordering

- Tools execute in the order they appear in the YAML `plugins:` list (top to bottom).

### Target Chaining

- If a tool is configured with `produces_targets: true` and it exits successfully, its stdout is normalized and merged into the current targets list.
- A derived targets file is written under `input/targets.<tool>.txt` and `{{targets_file}}` points at that derived file for subsequent tools in the same apex group.

### Timeouts and Cancellation

- Each plugin run has a per-tool timeout: `tool_timeout_sec` (set to `0` to disable timeouts).
- SkuntScan uses context cancellation to terminate subprocesses when possible.
- Ctrl+C cancels the root context, which cascades to all in-flight plugin runs.

### Fail-Fast Behavior

- If `fail_fast: true`, the first observed error cancels the shared run context.
- If `fail_fast: false`, SkuntScan attempts to run all enabled plugins and returns an aggregated error if any failed.

## Outputs

SkuntScan stores raw artifacts for each plugin run:

- Stdout: `<output_dir>/<apex_dir>/raw/<plugin>/<task_id>.out` (streamed during execution)
- Stderr: `<output_dir>/<apex_dir>/raw/<plugin>/<task_id>.err` (streamed during execution)
- Metadata (JSON): `<output_dir>/<apex_dir>/raw/<plugin>/<task_id>.json`

Metadata includes:

- plugin name
- task id
- exit code
- stdout file name
- stderr file name
- whether an error occurred

## Current Limitations (By Design)

- Plugin ordering is not yet stage-aware; enabled plugins execute without explicit dependency resolution.
- Pipeline variables like `{{alive_file}}` are not produced by the runner yet; those entries are typically kept `enabled: false` until the pipeline is implemented.

