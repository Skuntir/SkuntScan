# Security Policy

SkuntScan is intended for authorized security testing only. If you discover a security issue in SkuntScan itself (not in third-party tools it executes), please report it responsibly.

## Supported Versions

This repository does not currently publish formal releases. Security fixes are applied to the default branch.

If you need a supported release process (tags, signed artifacts, CVE workflow), open an issue requesting it.

## Reporting a Vulnerability

Please do not open public GitHub issues for security reports.

Instead:

1. Send an email to the maintainers with:
   - a clear description of the issue
   - impact assessment (what could go wrong)
   - minimal steps to reproduce
   - affected version/commit (if known)
2. If the issue involves secrets you may have exposed, rotate them immediately.

If you do not have a maintainer email address available, open a GitHub issue that only says you have a security report and request a private contact method. Do not include technical details in the issue.

## What to Expect

- Acknowledge receipt within 7 days when possible.
- Provide a status update when a fix is in progress.
- Coordinate a responsible disclosure timeline for high-impact issues.

## Scope

In scope:

- Vulnerabilities in this repository’s Go code and default configuration behavior
- Supply-chain issues caused by files published in this repository (example: malicious config changes)

Out of scope:

- Vulnerabilities in external tools (subfinder, nuclei, etc.)
- Vulnerabilities in targets scanned with SkuntScan
- Misconfiguration of third-party tooling

## Safe Defaults and Operational Guidance

- Do not store API keys or tokens in configs committed to git.
- Use least-privilege execution environments.
- Prefer running SkuntScan and third-party tools in isolated environments when possible.

