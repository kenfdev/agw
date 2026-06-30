# Base Environment Guidance Design

Date: 2026-06-30

## Summary

AGW should let users define personal, reusable development-environment guidance
that is automatically included when preparing a workspace. The guidance is
natural language, usually Markdown, not a shared Dockerfile template. It can
include reference snippets, repository links, and preferences, but AGW treats it
as instruction material for the agent that generates workspace files.

The feature adds a global default guidance file in AGW config and optional
workspace-specific guidance in each workspace definition. A workspace can add to
the global guidance or disable the global guidance entirely.

## Goals

- Let users manage common development-environment preferences in one place.
- Include that guidance in `agw workspace prepare` prompts and agent JSON.
- Keep Dockerfile and Compose generation project-specific and agent-assisted.
- Support workspace-specific additions.
- Let a workspace opt out of global guidance.
- Fail clearly when configured guidance files are missing or unreadable.

## Non-Goals

- Do not add Dockerfile templating.
- Do not merge or transform Dockerfile snippets.
- Do not run an LLM inside AGW.
- Do not install tools directly from AGW.
- Do not edit target project repositories.
- Do not add workspace-specific replacement of the global guidance in this
  slice; workspaces can include or exclude the global guidance and add local
  guidance.

## Configuration Model

Add optional `baseEnvironment` settings to global AGW config:

```yaml
workspaceRoots:
  - /path/to/agw-root
baseEnvironment:
  guidancePath: /Users/me/.config/agw/base-environment.md
```

Add optional `baseEnvironment` settings to workspace definitions:

```yaml
id: agw
name: AGW
workspace:
  dir: workspaces/agw
container:
  service: dev
  workdir: /workspace
baseEnvironment:
  includeGlobal: true
  guidancePath: environment.md
projects:
  - name: agw
    hostPath: /path/to/agw
    containerPath: /workspace
```

`includeGlobal` defaults to true when omitted. `guidancePath` is optional in
both places. If neither global nor workspace guidance is configured, AGW behaves
as it does today.

## Path Resolution

Global `baseEnvironment.guidancePath` is resolved as follows:

- Absolute paths are used as-is.
- Relative paths are resolved relative to the directory containing the
  `config.yaml` that was loaded.

Workspace `baseEnvironment.guidancePath` is resolved as follows:

- Absolute paths are used as-is.
- Relative paths are resolved relative to the directory containing that
  workspace's `agw.yaml`.

Resolved paths are read only during `workspace prepare`; `workspace new` only
writes the definition fields that the command supports.

## Prepare Behavior

`agw workspace prepare <workspace>` loads configured guidance and adds it to the
rendered prompt after workspace basics and before project file snapshots.

The prompt section should be explicit that this is adaptive guidance:

```text
## Base Environment Guidance

Use this guidance when generating the workspace Dockerfile and Compose files.
It is not a fixed template. Adapt it to the project's base image, package
manager, language runtime, and build constraints. Reference snippets are
examples, not content to paste blindly.

### Global Guidance
...

### Workspace Guidance
...
```

`--agent-json` continues to include the full prompt. It should also expose the
guidance metadata in a machine-readable field so agents can tell which guidance
files influenced the prompt without parsing Markdown.

Example shape:

```json
{
  "baseEnvironment": {
    "globalGuidancePath": "/Users/me/.config/agw/base-environment.md",
    "workspaceGuidancePath": "/agw-root/workspaces/agw/environment.md",
    "includeGlobal": true
  }
}
```

The exact Go struct names can follow existing CLI style.

## Error Handling

If a configured guidance file cannot be read, `workspace prepare` fails with a
message that identifies the failing path and whether it came from global or
workspace config.

AGW should not silently ignore a configured missing file. A missing configured
guidance file means the user intended the guidance to influence generated
workspace files, and continuing could produce the wrong environment.

If global guidance is configured but a workspace sets `includeGlobal: false`,
AGW does not read the global guidance file for that workspace. This allows a
workspace to opt out even if the global file is temporarily unavailable.

## CLI Surface

No new command is required for the first slice. Users can edit `config.yaml`,
`agw.yaml`, and guidance Markdown files directly.

Future commands may help initialize or inspect guidance, but they are outside
this design:

- `agw config set-base-environment <path>`
- `agw workspace environment set <workspace> <path>`
- `agw workspace environment show <workspace>`

## Testing

Focused tests should cover:

- Global config YAML load/save with `baseEnvironment.guidancePath`.
- Workspace definition YAML load/save with `baseEnvironment.includeGlobal` and
  `baseEnvironment.guidancePath`.
- Default `includeGlobal` behavior when the field is omitted.
- Relative path resolution for config-relative and workspace-relative guidance.
- `prepare.Render` output with global guidance, workspace guidance, both, and
  neither.
- `workspace prepare --agent-json` includes the rendered guidance and metadata.
- Missing configured guidance files fail clearly.
- Workspace `includeGlobal: false` avoids reading global guidance.

## Compatibility

The new fields are optional, so existing AGW config files and workspace
definitions continue to load. Saving a definition should preserve the new
base-environment settings and continue clearing legacy workspace fields as it
does today.

## Open Decisions

- Whether `workspace new` should gain flags for base environment guidance in a
  later slice.
- Whether `doctor` should report configured guidance problems before
  `workspace prepare`.
