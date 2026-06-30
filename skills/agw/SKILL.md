---
name: agw
description: Drive AGW workspace setup, diagnosis, lifecycle, and troubleshooting through the up-to-date local `agw` CLI. Use when a user wants to create, prepare, apply, start, stop, attach to, inspect, diagnose, or repair an AGW workspace, especially when project-specific Docker/container details are uncertain.
---

# AGW

Use the repository's current CLI as the source of truth. Do not duplicate or invent AGW behavior from memory.

## First Steps

1. Confirm the CLI surface before acting:
   - Prefer `go run ./cmd/agw --help` from this repository when working on AGW itself.
   - Use `agw --help` only when intentionally targeting the installed binary.
   - Read command-specific help before using less familiar commands.
2. Read `README.md` when workflow context is needed.
3. Run `agw doctor <workspace>` or `agw doctor --all` before lifecycle decisions when a workspace already exists.

## Operating Model

- Treat `doctor` output as the state machine and follow its next action unless there is a clear reason not to.
- Default to standalone sidecar mode. Target projects do not need Docker files, Compose files, devcontainer files, or external networks.
- Treat detected project Docker files as hints only.
- Add external networks only when the user explicitly wants the sidecar to reach already-running project services.
- Do not start, stop, or modify target project services unless the user specifically asks.
- Do not edit files inside target repositories as part of workspace preparation. AGW owns its workspace files.

## Common Flow

For a new project, prefer:

```bash
agw workspace new --from /path/to/project
agw workspace prepare <workspace> --output <workspace-dir>/prompt.md
agw workspace apply <workspace> <generated-dir>
agw start <workspace>
```

For an existing workspace, prefer:

```bash
agw doctor <workspace>
agw start <workspace>
agw stop <workspace>
```

Use `agw workspace network add <workspace> <network>` only after confirming that an external Docker network is intended.

## Error Handling

- If `doctor` reports `needs-prepare`, render the prompt and have the agent produce workspace files from that prompt.
- If `doctor` reports `needs-apply`, apply generated workspace files from the chosen generated directory.
- If `doctor` reports a missing external network, ask the user to start the base project services or remove the network selection. Do not manage those services implicitly.
- If the installed `agw` binary disagrees with this repository, prefer the repository command during development and call out the difference.
