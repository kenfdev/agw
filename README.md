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

## MVP Flow

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

By default, AGW stores user configuration in the OS user config directory
under `agw/config.yaml`. Set `AGW_CONFIG` to use a specific config file:

```bash
export AGW_CONFIG="$HOME/.config/agw/config.yaml"
```

## Base environment guidance

AGW can include personal development-environment guidance in workspace
preparation prompts. This is natural-language guidance for the agent that
generates workspace files, not a shared Dockerfile template.

Global guidance in `config.yaml`:

```yaml
baseEnvironment:
  guidancePath: /Users/me/.config/agw/base-environment.md
```

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
