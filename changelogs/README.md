# 2026-01-31

## Added

- `changelogs/` folder for dated change notes.
- `configs/external-enterprise.yaml` example pipeline config (direct tool binaries, no shell wrappers).
- `-regen-default-config` flag to overwrite the on-disk default config from the embedded template (`-regen-config` kept as an alias).
- Config placeholders: `{{stdout_file_<tool>}}` and `{{stderr_file_<tool>}}`.
- TUI terminal-width detection for stable boxed table rendering.

## Changed

- Docs reorganized to `docs/<topic>/README.md` for GitHub auto-rendering.
- `produces_targets: true` now normalizes and merges tool stdout into the current targets set (instead of replacing).
- Default config template lives at `cmd/SkuntScan/default/default.yaml` (embedded into the binary).
- Workflow `main.yml` now uses the TAG for release versioning.
- Raw output filenames now use `tool-YYYY_MM_DD_HH_MM.*` (collision-safe suffixes on reruns).
- TUI table headers and spacing: `DUR` renamed to `DURATION`, `TIME` column widened.

## Removed

- `skuntscan-merge` helper dependency; target merging is handled internally now.

## Fixed

- GitHub Actions release workflow tarball filename template (`${VERSION}`).
- Boxed TUI redraw artifacts that could appear as duplicated panels/borders on narrow terminals.

