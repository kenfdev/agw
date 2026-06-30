# Agent Instructions

- Keep repo context small. Read only the files needed for the task.
- Use the repo-local `agw` skill for AGW workspace setup, diagnosis, lifecycle, and troubleshooting.
- Treat the current CLI and tests as the source of truth; prefer `go run ./cmd/agw ...` while developing this repo.
- Run focused `go test` commands before claiming code or CLI behavior is fixed.
