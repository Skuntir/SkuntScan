# Configuration

SkuntScan reads configuration from a YAML file. The config describes global runtime limits and a list of plugins to execute.

## Top-Level Schema

```yaml
output_dir: .
concurrency: 16
tool_timeout_sec: 300
debug: false
verbose: false
fail_fast: false
plugins:
  - name: subfinder
    binary: $HOME/go/bin/subfinder
    enabled: true
    produces_targets: true
    flags:
      - -dL
      - "{{targets_file}}"
      - -silent
```

### Fields

- `output_dir` (string): Directory where all artifacts are written.
- `concurrency` (int): Max concurrent plugin runs.
- `tool_timeout_sec` (int): Per-plugin timeout in seconds. Use `0` to disable timeouts.
- `debug` (bool): Enables debug behavior (future expansion).
- `verbose` (bool): Enables verbose runtime logs (streams tool output).
- `fail_fast` (bool): Cancel the run on first error.
- `plugins` (list): Plugin entries to execute.

## Plugin Entries

Each plugin entry describes how to run one external tool:

- `name` (string, required): Registry key and output folder name.
- `binary` (string, required): Executable to run (must be on `PATH` or a full path).
- `enabled` (bool, optional): If `false`, plugin is skipped. Default is enabled.
- `produces_targets` (bool, optional): If `true`, the tool's stdout is normalized and merged into the current targets list for subsequent tools (only on successful exit).
- `timeout_sec` (int, optional): Per-tool timeout override in seconds. Use `0` to disable for this tool.
- `flags` (list of strings): Argument vector passed to the tool.

## Execution Order

Plugins execute in the order they appear in `plugins:` (top to bottom).

## Running Your Own Tools

SkuntScan treats each plugin entry as a subprocess invocation with an explicit argument vector. This means you can run anything that can be started as a process:

- custom internal binaries
- scripts (Python, Bash) if invoked explicitly
- vendor tools not shipped with SkuntScan

Example: run a Python script:

```yaml
  - name: my-python-check
    binary: /usr/bin/python3
    flags:
      - ./scripts/my_check.py
      - --targets
      - "{{targets_file}}"
      - --out
      - "{{output_dir}}/reports/my_check.json"
```

Example: run a Bash script:

```yaml
  - name: my-bash-check
    binary: /usr/bin/bash
    flags:
      - ./scripts/my_check.sh
      - "{{targets_file}}"
```

## Flags as an Argument Vector

SkuntScan does not run your command through a shell by default. It executes:

```
binary + flags[]
```

So you should model your flags as discrete tokens:

- Good:
  - `["--output", "file.json"]`
  - `["-severity", "info,low,medium,high,critical"]`
- Risky:
  - `["--output file.json"]` (some tools accept this, others do not)

## Variable Substitution

Arguments support simple string substitution:

- `{{targets_file}}` → current targets file for this apex group (may be rewritten by producing tools)
- `{{base_targets_file}}` → original `<output_dir>/<apex_dir>/input/targets.txt`
- `{{targets_file_<tool>}}` → explicit targets file for a tool name (example: `{{targets_file_subfinder}}`)
- `{{stdout_file_<tool>}}` → stdout file path for the most recent run of a tool (example: `{{stdout_file_katana}}`)
- `{{stderr_file_<tool>}}` → stderr file path for the most recent run of a tool (example: `{{stderr_file_katana}}`)
- `{{output_dir}}` → absolute output directory for the current apex group

Substitution is applied to every `flags` element.

## Disabled Entries for Future Stages

Some example configs include tools set to `enabled: false` because they require pipeline-produced inputs (e.g., alive URL lists, resolved host lists, URL corpora). The intent is:

- keep the full pipeline “blueprint” in config
- enable stages as the runner gains support for producing intermediate artifacts and variables

## Linux Defaults

- Default config location (when `-config` is not provided):
  - `$XDG_CONFIG_HOME/SkuntScan/conf/default.yaml`, or
  - `~/.config/SkuntScan/conf/default.yaml`
- On Linux, enabled plugin `binary` values must resolve to an absolute path after env/user expansion (example: `/usr/bin/httpx` or `$HOME/go/bin/httpx`).

## Regenerating the Default Config

If the embedded default config template changes, you can overwrite the on-disk default config:

```bash
SkuntScan -regen-default-config
```

## Output Directory Resolution

- If `-out` is provided, SkuntScan writes outputs there.
- If `-targets` points to a file, SkuntScan defaults to writing outputs next to that targets file.
- Otherwise, it uses the current working directory or `output_dir` (depending on CLI wiring).

## Multiple Config Files

You can keep multiple config files in `~/.config/SkuntScan/conf/` and select one at runtime:

```bash
SkuntScan -config ~/.config/SkuntScan/conf/allconfigs.yaml -targets ./targets.txt
```

