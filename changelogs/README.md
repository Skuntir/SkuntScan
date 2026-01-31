# 2026-01-31

## Added

- Boxed table-style console UI for tool progress in non-verbose mode.
- `configs/external-enterprise.yaml` example external recon pipeline (direct tool binaries, no shell wrappers).
- `-regen-default-config` flag to overwrite the on-disk default config from the embedded template.
- New config placeholders: `{{stdout_file_<tool>}}` and `{{stderr_file_<tool>}}`.

## Changed

- `produces_targets: true` now normalizes and merges tool stdout into the current targets set (instead of replacing).
- Default config template moved under `cmd/SkuntScan/default/default.yaml` (embedded into the binary).

