# AGW

## Install

Install the latest released AGW binary:

```bash
curl -fsSL https://raw.githubusercontent.com/kenfdev/agw/main/install.sh | sh
```

Install a pinned version:

```bash
curl -fsSL https://raw.githubusercontent.com/kenfdev/agw/main/install.sh | AGW_VERSION=v0.1.0 sh
```

By default, the installer uses `/usr/local/bin` when writable and falls back to
`$HOME/.local/bin`. Set `AGW_INSTALL_DIR` to choose a directory:

```bash
curl -fsSL https://raw.githubusercontent.com/kenfdev/agw/main/install.sh | AGW_INSTALL_DIR="$HOME/bin" sh
```

## Use AGW with an LLM

AGW is designed to make workspace setup and diagnosis easy to delegate to an
LLM. You do not need to memorize every lifecycle command. In normal use, you
tell the LLM what you want, explicitly ask it to use the `agw` skill, and let
that skill drive the current `agw` CLI.

### One-time local setup

First choose where AGW should keep your personal workspace definitions and
generated files:

```bash
agw config init --root /path/to/personal/agw-root
```

This root belongs to you. It is separate from the target repositories you work
on.

### Initialize a workspace

Minimal prompt:

```text
Set up an AGW workspace for this repository using the agw skill.
```

More explicit prompt:

```text
Set up an AGW workspace for this repository using the agw skill. Use the current repository as the project, prefer standalone sidecar mode, and do not modify the target repository.
```

The LLM should use the `agw` skill to create a workspace definition, prepare the
workspace prompt, apply generated workspace files, and start from the current
CLI behavior instead of guessing project-specific Docker details.

### Diagnose an existing workspace

Minimal prompt:

```text
Diagnose my AGW workspace using the agw skill and tell me the next required action.
```

More explicit prompt:

```text
Diagnose my AGW workspace using the agw skill. Use agw doctor --json as the source of truth, follow the reported next action, and do not start, stop, or modify project services unless I ask.
```

The important part is that `doctor --json` is the state machine. The LLM should
inspect that output before deciding whether the workspace needs preparation,
generated files need to be applied, or the sidecar can be started.

### What the skill uses behind the scenes

These are the main CLI commands the `agw` skill may use while handling your
request:

| Situation | Representative CLI |
| --- | --- |
| Create a workspace definition | `agw workspace new ...` |
| Render an agent-readable preparation packet | `agw workspace prepare <workspace> --agent-json` |
| Apply generated workspace files | `agw workspace apply <workspace> <generated-dir>` |
| Diagnose current state | `agw doctor <workspace> --json` |
| Start the workspace when ready | `agw start <workspace>` |

## Release

Releases are published from semantic version tags:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The tag workflow runs tests, builds macOS and Linux binaries with GoReleaser,
uploads checksums, and publishes a GitHub Release.

Validate the release configuration locally before tagging:

```bash
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
```

## Docker Boundary

AGW does not embed Docker, the Docker Engine, or a Docker SDK. It prepares
workspace definitions and generated files, then either prints guidance or calls
the user's installed `docker` CLI for convenience commands such as
`docker compose build`, `docker compose up`, and `docker compose exec`.

Docker remains an external runtime owned by the user. AGW should stay a thin
workspace preparation and command-assistance layer, not a container
orchestrator.

## Manual CLI flow

You can also run the lifecycle yourself without an LLM. This is useful for
understanding the underlying flow or for debugging a workspace by hand.

```bash
agw config init --root /path/to/personal/agw-root
agw workspace new --root /path/to/personal/agw-root --id agw --name AGW --workspace-dir workspaces/agw --project agw=/path/to/agw:/workspace --service dev --workdir /workspace
agw workspace prepare agw --output prompt.md
agw workspace apply agw ./generated
agw build agw
agw up agw
agw attach agw
```

Workspace definitions separate host-side AGW files from paths inside the
container:

```yaml
id: agw
name: AGW
workspace:
  dir: workspaces/agw
container:
  service: dev
  workdir: /workspace
projects:
  - name: agw
    hostPath: /path/to/agw
    containerPath: /workspace
```

If starting the workspace needs a wrapper command, set `lifecycle.start`.
`agw start` runs it from the workspace directory instead of the default
`docker compose up -d`, then keeps the normal attach behavior:

```yaml
lifecycle:
  start: op run --env-file=.env.1password -- docker compose up -d
```

By default, AGW stores user configuration in the OS user config directory
under `agw/config.yaml`. Set `AGW_CONFIG` to use a specific config file:

```bash
export AGW_CONFIG="$HOME/.config/agw/config.yaml"
```

`workspaceRoots` entries may use `~/`, `$HOME`, or `${HOME}`. AGW expands
those values and cleans the resulting path when loading the config.

## Base environment guidance

AGW can include personal development-environment guidance in workspace
preparation prompts. This is natural-language guidance for the agent that
generates workspace files, not a shared Dockerfile template.

For a personal default, place your base container prompt in a Markdown file and
point AGW at it from your user config. The LLM should do this during initial
AGW setup when you ask it to configure your workspace preferences.

Recommended location:

```text
~/.config/agw/base-environment.md
```

Global guidance in `~/.config/agw/config.yaml`:

```yaml
baseEnvironment:
  guidancePath: base-environment.md
```

The guidance file should describe preferences the LLM adapts into generated
workspace files. For example, it can tell the LLM which tools you normally
want available and any constraints it should preserve:

````markdown
# Personal base container prompt

Install the tools I commonly use in development containers:

- git
- openssh-client
- tmux
- fzf
- ripgrep
- fd
- gh
- neovim

Prefer project-declared runtime versions over my defaults.
Do not bake secrets into the image.
````

Workspace-specific guidance in `agw.yaml`:

```yaml
baseEnvironment:
  includeGlobal: true
  guidancePath: environment.md
```

Set `includeGlobal: false` for a workspace that should ignore the global
guidance. Relative global paths are resolved from the config file directory.
Relative workspace paths are resolved from the workspace `agw.yaml` directory.

## Diagnose workspace state

Use doctor to see what a workspace needs next:

```bash
agw doctor agw --config "$AGW_TEST_CONFIG"
agw doctor --all --config "$AGW_TEST_CONFIG"
```

Agents should use the JSON protocol instead of parsing human output:

```bash
agw doctor agw --json --config "$AGW_TEST_CONFIG"
agw workspace prepare agw --agent-json --config "$AGW_TEST_CONFIG"
```

The state-first TUI shows the same model:

```bash
agw tui --config "$AGW_TEST_CONFIG"
```
