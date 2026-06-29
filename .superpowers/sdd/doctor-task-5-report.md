# Task 5: State-First TUI

Implemented the TUI so it renders doctor report state instead of only action output, and added `r` to refresh the selected workspace report.

## What changed

- Added report-backed TUI model construction with `NewModelWithReports`.
- Extended the TUI model to track both workspaces and doctor reports.
- Rendered workspace rows with state and the first failing check.
- Rendered selected report details, checks, and log output in the TUI.
- Added `r` refresh handling to re-run doctor diagnostics for the selected workspace.
- Updated `agw tui` startup to precompute initial reports and pass them into the TUI.
- Added `tuiActions.Refresh` using `doctor.Diagnose`.
- Added tests for initial report rendering and refresh behavior.

## Verification

- `go test ./internal/tui -run 'TestModelInitialViewShowsWorkspaceState|TestModelRefresh' -v`
- `go test ./internal/tui ./internal/cli -v`
- `go test ./...`

## Concerns

None.
