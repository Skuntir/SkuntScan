# Contributing to SkuntScan

Thanks for taking the time to contribute.

## Ground Rules

- Be respectful and constructive. See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
- Do not submit or request scan results, target lists, credentials, or other sensitive data.
- No secrets in commits. If you accidentally commit a secret, rotate it and contact maintainers.

## What to Contribute

Good first contributions:

- Documentation fixes (README, docs/)
- Improvements to error messages and UX
- New example configs and plugin templates
- Tests for runner/plugin behavior

Larger contributions:

- Better target parsing and apex domain extraction
- Stage-aware pipeline (assets → resolved → alive → urls → scans)
- Reporting aggregation (JSON/SARIF/Markdown)

## Development Setup

Requirements:

- Go 1.21+
- Linux for runtime behavior (the CLI exits on non-Linux)

Clone and run tests:

```bash
go test ./...
```

## Code Style

- Keep changes small and focused.
- Prefer clear names over cleverness.
- Match existing patterns in the surrounding package.

## Adding / Updating Plugins

Most “new tools” work requires no Go changes:

1. Edit a config YAML.
2. Add/update plugin entries under `plugins:`.
3. Use absolute Linux paths in `binary` for enabled plugins.

If you add new substitution variables or change config schema, update:

- README.md
- docs/config/README.md and docs/semantics/README.md
- tests as needed

## Pull Request Checklist

- Tests pass: `go test ./...`
- No secrets committed
- Docs updated if behavior or flags changed
- Changes are scoped to the PR topic

## Reporting Security Issues

Please do not open public issues for security bugs. See [SECURITY.md](SECURITY.md).

