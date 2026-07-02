---
name: agw
description: Drive AGW workspace setup, diagnosis, refresh, lifecycle, and troubleshooting through the up-to-date local `agw` CLI. Use when a user asks an agent to create, prepare, apply, update, refresh, rebuild, start, stop, attach to, inspect, diagnose, or repair an AGW workspace, especially when project-specific Docker/container details are uncertain or personal base-environment guidance changed.
---

# AGW

Use the repository's current CLI as the source of truth. Do not duplicate or invent AGW behavior from memory.

The human interface is natural language. Users should be able to say things like
"set up AGW here", "refresh this workspace with my new base prompt", or "start
my workspace". The agent uses JSON and agent-oriented CLI output internally.

## First Steps

1. Confirm the CLI surface before acting:
   - Prefer `go run ./cmd/agw --help` from this repository when working on AGW itself.
   - Use `agw --help` only when intentionally targeting the installed binary.
   - Read command-specific help before using less familiar commands.
2. Read `README.md` when workflow context is needed.
3. Run `agw doctor <workspace> --json` or `agw doctor --all --json` before lifecycle decisions when a workspace already exists.
4. Configured `workspaceRoots` may use `~/`, `$HOME`, or `${HOME}`; the CLI expands and cleans these paths when loading config.

## Operating Model

- Treat `doctor --json` output as the state machine and follow its next action unless there is a clear reason not to.
- Default to standalone sidecar mode. Target projects do not need Docker files, Compose files, devcontainer files, or external networks.
- Treat detected project Docker files as hints only.
- Add external networks only when the user explicitly wants the sidecar to reach already-running project services.
- Do not auto-detect or manage target project services unless the user specifically asks or `projects[].lifecycle.start` / `projects[].lifecycle.stop` is explicitly configured.
- Do not edit files inside target repositories as part of workspace preparation. AGW owns its workspace files.
- Generate workspace files in a temporary directory outside the AGW workspace directory before `workspace apply`; `apply` rejects generated directories that overlap the workspace.
- Do not change Compose image names as a workaround for stale images. Diagnose stale containers/images, then rebuild or remove the old image intentionally.

## Common Flow

For a single-project workspace, when the user asks the agent to create or set
up an AGW workspace, prefer:

```bash
agw workspace new --from /path/to/project
agw workspace prepare <workspace> --agent-json
# Generate workspace files in a temporary directory outside the workspace.
agw workspace apply <workspace> <temp-generated-dir>
agw start <workspace>
```

`workspace new --from` may suggest the workspace directory from configured path
mappings. For example, a project under `~/ghq/github.com/kenfdev/agw` with a
mapping from `~/ghq` to `workspaces` should become
`<AGW_ROOT>/workspaces/github.com/kenfdev/agw`. Treat this as a default
suggestion, not a user intent override.

For a workspace that groups multiple projects, do not rely on the single
project `--from` default. Ask for or infer a meaningful group id and workspace
directory, then create the definition explicitly with repeated `--project`
flags:

```bash
agw workspace new \
  --root <AGW_ROOT> \
  --id <workspace-id> \
  --name <workspace-name> \
  --workspace-dir workspaces/<chosen-group-path> \
  --project api=/path/to/api:/workspace/api \
  --project web=/path/to/web:/workspace/web \
  --service dev \
  --workdir /workspace
```

When the user has not specified a group directory, propose one based on the
shared source-root-relative path, organization, product, or repo family, and
make clear it can be changed with `--workspace-dir`. Preserve each target
project's host path; do not move, rename, or edit target repositories.

For an existing workspace lifecycle request, prefer:

```bash
agw doctor <workspace> --json
agw start <workspace>
agw stop <workspace>
```

In `agw`, press `t` to start the selected workspace in daemon mode using
the same readiness checks and `lifecycle.start` handling as `agw start -d`.

If a workspace `agw.yaml` contains `lifecycle.start`, `agw start` runs that
shell command from the workspace directory instead of the default
`docker compose up -d` startup step. This is intended for wrappers such as:

```yaml
lifecycle:
  start: op run --env-file=.env.1password -- docker compose up -d
```

If projects have explicit lifecycle commands, `agw start` runs each
`projects[].lifecycle.start` from that project's `hostPath` in project order
after readiness checks and before starting or attaching to the AGW sidecar.
`agw start` fails immediately if a project start command fails. `agw stop`
stops the AGW sidecar first, then runs each `projects[].lifecycle.stop` from
the project `hostPath` in reverse project order. Project stop failures do not
prevent later project stop commands from running, but `agw stop` returns an
error if any configured project stop command fails.

```yaml
projects:
  - name: api
    hostPath: /path/to/api
    containerPath: /workspace/api
    lifecycle:
      start: docker compose up -d
      stop: docker compose down
```

For an existing workspace that should be refreshed from changed global
base-environment guidance, dotfiles guidance, tool defaults, or generated
workspace files:

```bash
agw doctor <workspace> --json
agw workspace prepare <workspace> --agent-json
# Generate updated workspace files from the prompt, preserving project-specific choices.
agw workspace apply <workspace> <temp-generated-dir>
```

After applying, explain whether a rebuild or restart is needed. Only run
`agw start`, `agw stop`, `agw up`, `agw down`, or Docker lifecycle commands when
the user asked for that lifecycle action or it is clearly required and low risk.

Use `agw workspace network add <workspace> <network>` only after confirming that an external Docker network is intended.

## Sidecar Generation Guidance

When generating Docker/Compose files from `workspace prepare --agent-json`:

- Prefer the user's global base-environment guidance when present; do not silently omit mandatory personal tooling such as Tailscale or dotfiles.
- Project bind mount sources in generated Compose may use absolute paths, `~/`, `$HOME`, or `${HOME}`; `workspace apply` and `doctor` expand and clean those sources when checking them against `projects.hostPath`.
- For Ubuntu 24.04 based sidecars, use the existing `ubuntu` user unless there is a clear project reason to create another user. Do not unconditionally create UID/GID 1000 users; `ubuntu:24.04` already provides `ubuntu:1000`.
- If Tailscale is installed through the user's tools feature, Compose must also run the Tailscale entrypoint and provide runtime requirements: `TS_AUTHKEY` or `TS_AUTH_KEY`, `/dev/net/tun`, `NET_ADMIN`, `MKNOD`, and persistent `/var/lib/tailscale` state.
- Prefer a workspace-local `.env.1password` for Tailscale auth key references when the user's base guidance uses 1Password. Do not write real auth keys into generated files.
- If workspace startup needs secrets or another wrapper, set `lifecycle.start` in `agw.yaml` to the exact one-line command AGW should run from the workspace directory.
- If project-owned Docker services should start or stop with AGW, set `projects[].lifecycle.start` and `projects[].lifecycle.stop` to exact one-line commands AGW should run from each project host path. Do not add these commands based only on detected Docker files without user intent.
- Tailscale SSH authorization is controlled by tailnet policy, not container `authorized_keys`. If SSH fails with "tailnet policy does not permit", inspect `tailscale status --json`, node tags, requested login user, and the tailnet `ssh` ACL/grants.
- If Docker reports an entrypoint or binary missing after Compose changes, suspect a stale image or unrecreated container first. Keep the image name stable, then rebuild with `docker compose up -d --build --force-recreate` or remove the old image/container deliberately.
- Dotfiles cloned during image build need BuildKit SSH forwarding. If build fails with an empty SSH agent socket, ask the user to ensure `SSH_AUTH_SOCK` is set and `ssh-add -l` works in their terminal.

## Human Request Examples

Map user intent to CLI use without teaching users to run agent-oriented flags:

- "Refresh this workspace with my latest base prompt" -> run `doctor --json`,
  render `workspace prepare --agent-json`, regenerate workspace files using the
  packet prompt, apply them, and report what changed.
- "Update my current AGW container tooling" -> treat this as a refresh from
  base-environment guidance unless the user names a specific tool change.
- "Make this workspace use my dotfiles/devcontainer-feature defaults" -> prepare
  with global base guidance included, generate files that preserve project
  runtime versions, apply them, then advise on rebuild/start.
- "Diagnose why this workspace will not start" -> run `doctor --json`, follow
  the reported next action, and use command-specific help before unfamiliar
  operations.
- "Start my workspace" -> diagnose first when state is unknown, then run configured
  project lifecycle starts before starting or attaching to the AGW workspace sidecar.

## Error Handling

- If `doctor --json` reports `needs-prepare`, render the agent packet with `agw workspace prepare <workspace> --agent-json` and produce workspace files from its `prompt`.
- If `doctor` reports `needs-apply`, apply generated workspace files from the chosen generated directory.
- If `doctor` reports a missing external network, ask the user to start the base project services, configure explicit project lifecycle commands, or remove the network selection. Do not manage those services implicitly.
- If the installed `agw` binary disagrees with this repository, prefer the repository command during development and call out the difference.
