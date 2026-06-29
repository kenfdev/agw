Status: DONE

Commits created (short SHA + subject)
- e72004f feat: implement Go module and CLI root command skeleton

One-line test summary
- `go mod tidy && go test ./... && go run ./cmd/agw --help` passed; `go test ./internal/cli -run TestRootCommandShowsHelp -v` failed before module creation as expected.

Concerns, if any
- None.

Report file path
- `/Users/fukuyamaken/ghq/github.com/kenfdev/agw/.superpowers/sdd/task-1-report.md`
