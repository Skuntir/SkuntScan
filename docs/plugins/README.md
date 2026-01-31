# Plugins

SkuntScan plugins are adapters that execute work. Today, the primary plugin type is `ShellPlugin`, which runs an external binary using `os/exec`.

## Plugin Lifecycle

1. CLI loads plugin entries from YAML.
2. Each entry is registered into a runtime registry by name.
3. Runner looks up plugins by name and executes enabled entries.
4. Each execution returns a `Result` that is persisted to disk.

## ShellPlugin

ShellPlugin behavior:

- Executes `binary` with the argument vector from `flags`.
- Captures stdout and stderr (and can stream to provided writers during execution).
- Returns an exit code when available.
- Respects context cancellation and timeouts.

## How Custom Tools Work

When you add a tool in YAML, SkuntScan creates a `ShellPlugin` using:

- `name` → registry key and output folder
- `binary` → executable
- `flags` → argument vector

At runtime the runner:

1. Writes normalized targets to `<output_dir>/<apex_dir>/input/targets.txt`
2. Substitutes variables inside `flags`
3. Runs `binary` with `flags` as-is (no shell)
4. Streams raw artifacts under `<output_dir>/<apex_dir>/raw/<name>/` during execution and writes metadata at the end

## Writing Your Own Plugin (Go)

If YAML-only isn’t enough, you can implement your own plugin in Go. This is the path for “native” stages like:

- parsing and normalizing outputs into canonical files
- emitting structured results (JSON/SARIF/Markdown) without scraping stdout
- building a stage-aware pipeline (assets → resolved → alive → urls → scans)

### Step 1: Implement the Interface

Implement `plugin.Plugin` from:

- [plugin.go](../../internal/plugin/plugin.go)

The contract is:

- Input: `Task` (argument vector, optional writers, and an optional TaskID)
- Output: `Result` (stdout/stderr/exit/err)

### Step 2: Register It

Register your plugin in the registry from:

- [main.go](../../cmd/SkuntScan/main.go)

Today the CLI registers all config entries as shell plugins. A native plugin can be added by registering it under a unique `name` and ensuring the runner references that name in the config.

### Step 3: Decide What to Persist

SkuntScan always persists the raw plugin result. For richer outputs:

- have your plugin write additional files into `output_dir` (e.g., `assets/all.txt`)
- optionally keep stdout minimal and rely on files for downstream stages

## Suggested Extension Pattern (Stage-Aware Pipeline)

If you start implementing more stages:

- define canonical filenames per stage (e.g., `assets/all.txt`, `resolved/final.txt`, `http/alive.txt`)
- add new template variables (e.g., `{{alive_file}}`) once the runner produces them
- implement dependency ordering so a stage runs only after its inputs exist

## Adding a New Tool

1. Add a plugin entry to your config YAML:
   - Pick a unique `name`.
   - Set `binary` to an absolute Linux path after env/user expansion (for enabled plugins).
   - Add a `flags` list.
2. Ensure the binary exists at that path on your system.
3. Run SkuntScan and inspect `<apex_dir>/raw/<plugin>/` outputs.

## Best Practices

- Prefer deterministic outputs: always include flags that avoid interactive prompts.
- Keep argument lists explicit in YAML instead of relying on shell operators.
- Store intermediate artifacts as files so later stages can be represented as new variables.

## Useful Variables for Chaining

In addition to targets variables, you can reference the most recent output files for a given tool:

- `{{stdout_file_<tool>}}` (example: `{{stdout_file_katana}}`)
- `{{stderr_file_<tool>}}` (example: `{{stderr_file_katana}}`)

