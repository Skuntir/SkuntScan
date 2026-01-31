# Container Deployments

SkuntScan is an orchestrator. External tools (subfinder, nuclei, etc.) are dependencies of the runtime environment. If you run SkuntScan in a container, you must also provide those tools inside that container image or replace plugin `binary` paths accordingly.

## Local Docker Compose

From a working directory containing `targets.txt`:

```bash
docker compose -f deploy/compose/docker-compose.yaml up --build
```

This mounts:

- `${PWD}` to `/work` (targets and outputs live here)
- `${HOME}/.config/SkuntScan/conf` to `/config` (read-only config)

## Azure / Hosted Runtimes

Use `deploy/compose/docker-compose.azure.yaml` as a template for platforms that accept compose-like configuration. It assumes the platform provides:

- a working directory volume at `/work`
- a config volume at `/config`

You can override mount points via:

- `WORK_DIR`
- `CONFIG_DIR`
