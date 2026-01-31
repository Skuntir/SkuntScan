# Error Model

SkuntScan treats errors as first-class outputs. Each plugin run produces raw artifacts, and the overall program exit code reflects whether the run completed successfully.

## Error Categories

### Configuration Errors

Raised before execution:

- Invalid or missing required config fields
- Output directory cannot be created or written to
- Plugin entry missing `name` or `binary`

These errors are fatal and no plugins are executed.

### I/O Errors

Raised during execution:

- Failed writes of stdout or metadata files
- Failed creation of per-plugin output directories

If `fail_fast` is enabled, the run is cancelled. Otherwise, SkuntScan continues attempting other plugins and returns an aggregated error.

### External Tool Errors

Raised during execution:

- Subprocess cannot be started (binary not found, permission error)
- Subprocess returns non-zero exit code
- Subprocess is terminated due to timeout or cancellation

SkuntScan records:

- stdout (if any)
- stderr
- exit code (when available)

Raw outputs are persisted as:

- `<output_dir>/<apex_dir>/raw/<plugin>/<task_id>.out`
- `<output_dir>/<apex_dir>/raw/<plugin>/<task_id>.err`
- `<output_dir>/<apex_dir>/raw/<plugin>/<task_id>.json` (references file names + exit code)

## Fail-Fast vs Best-Effort

- `fail_fast: true`:
  - First observed error cancels the shared context.
  - In-flight tools receive cancellation/timeout and may stop early.
  - SkuntScan exits with a non-zero code.

- `fail_fast: false`:
  - SkuntScan attempts to run all enabled plugins.
  - It returns a non-nil error if any plugin failed.

## Exit Codes

- `0`: run completed and all enabled plugins executed successfully.
- `1`: run completed with at least one error (or fail-fast triggered).
- `2`: configuration or argument error (e.g., missing `-targets`, invalid config).

