# Agent Instructions

- Keep repo context small. Read only the files needed for the task.
- Use the repo-local `agw` skill for AGW workspace setup, diagnosis, lifecycle, and troubleshooting.
- Treat the current CLI and tests as the source of truth; prefer `go run ./cmd/agw ...` while developing this repo.
- Keep the repo-local `agw` skill aligned with CLI behavior. When changing workspace setup, lifecycle, config, prompts, generated-file flow, or other agent-facing behavior, update the skill in the same work before claiming the change is complete.
- Run focused `go test` commands before claiming code or CLI behavior is fixed.
- Do not commit agent-created design notes, plans, or specs unless explicitly asked.
