# AGW

## Docker Boundary

AGW does not embed Docker, the Docker Engine, or a Docker SDK. It prepares
workspace definitions and generated files, then either prints guidance or calls
the user's installed `docker` CLI for convenience commands such as
`docker compose build`, `docker compose up`, and `docker compose exec`.

Docker remains an external runtime owned by the user. AGW should stay a thin
workspace preparation and command-assistance layer, not a container
orchestrator.

## MVP Flow

```bash
agw config init --root /path/to/personal/agw-root
agw workspace new --root /path/to/personal/agw-root --id agw --name AGW --storage workspaces/github.com/kenfdev/agw --project agw=/path/to/agw:/workspace --service dev --workspace-root /workspace
agw workspace prepare agw --output prompt.md
agw workspace apply agw ./generated
agw build agw
agw up agw
agw attach agw
```

## Diagnose workspace state

Use doctor to see what a workspace needs next:

```bash
agw doctor agw --config "$AGW_TEST_CONFIG"
agw doctor --all --config "$AGW_TEST_CONFIG"
```

The state-first TUI shows the same model:

```bash
agw tui --config "$AGW_TEST_CONFIG"
```
