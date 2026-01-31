# Invariants

SkuntScan enforces the invariants below before executing plugins. If any invariant fails, the run aborts with a configuration error.

## Configuration Invariants

- `output_dir` is non-empty.
- `output_dir` is writable (SkuntScan can create directories and write files).
- `concurrency >= 1`.
- `tool_timeout_sec >= 0` (`0` disables timeouts).

## Plugin Invariants

For each plugin entry:

- `name` is non-empty.
- `binary` is non-empty.
- On Linux, enabled plugin `binary` values must resolve to an absolute path after env/user expansion.
- `flags` may be empty (some tools read from stdin or are invoked in a later stage).

## Runtime Invariants

During execution SkuntScan aims to preserve these properties:

- Tools execute in config order (top to bottom) for each apex group.
- Every plugin run produces a metadata JSON file, even when stdout/stderr are empty.
- Task IDs are unique enough to avoid collisions in typical usage (timestamp with nanoseconds).

