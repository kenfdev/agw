# AGW Start/Stop Workflow Direction

## Goal

Make starting development smooth for container-based developers without turning
AGW into a Docker Compose orchestrator.

Daily workflow target:

```bash
agw workspace new --from /path/to/project
agw start <workspace>
agw stop <workspace>
```

AGW owns the sidecar development workspace. Target project services remain owned
by the target project.

## Primary Use Case

A developer has one or more local project repositories and wants a disposable
development sidecar container that:

- mounts project repositories into a predictable workspace root;
- can start even when the target project has no Docker setup;
- can optionally join an existing Docker network when the target project already
  runs services with Compose;
- explains missing prerequisites clearly;
- avoids modifying files in the target repositories.

## Product Rules

1. Default to standalone sidecar mode.
2. Do not require Docker files, Compose files, devcontainer files, or external
   Docker networks in the target project.
3. Treat target-project Docker files as suggestions, not decisions.
4. External networks are optional and user-selected.
5. Missing external networks should be diagnosed with clear next steps, not
   automatically managed.
6. `agw start` and `agw stop` should only operate on AGW-owned workspace files
   and containers.
7. Do not add dependency orchestration until real usage proves it is necessary.

## Modes

### Standalone Sidecar

Used when the target project has no container setup, or when the user wants AGW
to provide an isolated dev container.

Expected behavior:

- no external network required;
- generated Compose file uses AGW-owned services and default Compose networking;
- `agw start <workspace>` builds, starts, and attaches to the sidecar;
- `agw stop <workspace>` stops the sidecar.

### Attached Sidecar

Used when the target project already has services running in another Compose
project, and the AGW sidecar needs network access to those services.

Expected behavior:

- user explicitly selects one or more external networks;
- `agw start <workspace>` checks that those networks exist;
- if a selected network is missing, AGW stops and prints guidance;
- AGW does not start or stop the base project services.

Example failure guidance:

```text
Cannot start workspace api-agent.

Missing external network:
  api_default

This usually means the base project services are not running.

Try:
  cd /Users/me/src/api
  docker compose up -d
  agw start api-agent
```

## Detection Strategy

AGW may scan target projects for obvious container-related files:

- `compose.yaml`
- `docker-compose.yaml`
- `Dockerfile`
- `.devcontainer/devcontainer.json`

The scan result should only guide prompts or hints. It must not decide behavior
automatically because these files may be stale, custom, incomplete, or unrelated
to the intended local workflow.

Default behavior should be standalone sidecar.

## Rejected For Now

Do not add `--with-deps` or explicit dependency orchestration yet.

Reasons:

- separate Compose projects do not have a safe ownership relationship;
- inferred dependency start/stop behavior is unreliable;
- stopping base project services may surprise users or affect other workspaces;
- dependency schema, ordering, partial startup, and failure behavior add
  significant complexity;
- the core value is better diagnosis and a smooth sidecar workflow, not managing
  every local service stack.

If this becomes necessary later, only support explicitly declared dependencies.
Do not infer them from external networks.

## Implementation Status

### Implemented: `agw start <workspace>`

Use the existing doctor/state model to drive the next safe action.

Implemented behavior:

- validate workspace definition and project paths;
- validate generated workspace files;
- check external networks only when configured;
- if the workspace can start, run build, `docker compose up -d`, and attach;
- if already running, attach;
- if blocked, print the reason and next action.

### Implemented: `agw stop <workspace>`

Stop only AGW-owned sidecar containers for the workspace.

Implemented as `docker compose stop`. The existing `agw down` command remains
available for stronger cleanup with `docker compose down`.

### Implemented: Improve `doctor` Output

Make standalone and attached modes explicit:

```text
external networks
  none required
```

or:

```text
external networks
  api_default  missing
```

Missing external networks should produce actionable guidance instead of feeling
like an AGW configuration failure.

### Implemented: `workspace new --from <path>`

Create a workspace definition from a project path with standalone sidecar as the
default.

Detection should identify possible Docker files and present them as hints. It
should not require or assume target project container settings.

### Implemented: `workspace network add <workspace> <network>`

Add user-selected external networks explicitly. AGW does not infer network
ownership from target project files.

### Implemented: Update Preparation Prompt

The prompt generated by `agw workspace prepare` should explicitly say:

- generate standalone workspace files when no external networks are selected;
- do not require Docker settings in the target project;
- do not edit target project files;
- use external networks only when selected in the AGW workspace definition.

## Open Questions

- Should network hints live in `agw.yaml`, or only be printed from detection?
- Should `workspace new --from` later gain an interactive mode, or stay
  non-interactive with safe defaults?
- Should AGW add a command for listing candidate Docker networks from the current
  Docker daemon?
