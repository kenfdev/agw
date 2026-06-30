package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kenfdev/agw/internal/doctor"
	"github.com/kenfdev/agw/internal/workspace"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	var configPath string
	var all bool
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "doctor [workspace]",
		Short: "Diagnose workspace state",
		Args: func(cmd *cobra.Command, args []string) error {
			if all {
				if len(args) > 0 {
					return fmt.Errorf("cannot specify workspace when --all is set")
				}
				return nil
			}
			if len(args) != 1 {
				return fmt.Errorf("requires exactly one workspace argument unless --all is set")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				items, err := listDefinitions(configPath)
				if err != nil {
					return err
				}
				sort.Slice(items, func(i, j int) bool { return items[i].Definition.ID < items[j].Definition.ID })
				if jsonOut {
					reports := make([]doctor.Report, 0, len(items))
					for _, located := range items {
						reports = append(reports, diagnoseWorkspace(cmd, located, io.Discard))
					}
					return writeJSON(cmd.OutOrStdout(), reports)
				}
				for _, located := range items {
					if err := runDoctorCommand(cmd, located); err != nil {
						return err
					}
				}
				return nil
			}
			located, err := findDoctorDefinition(configPath, args[0])
			if err != nil {
				return err
			}
			if jsonOut {
				return writeJSON(cmd.OutOrStdout(), diagnoseWorkspace(cmd, located, io.Discard))
			}
			return runDoctorCommand(cmd, located)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	cmd.Flags().BoolVar(&all, "all", false, "diagnose all known workspaces")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print machine-readable JSON")
	return cmd
}

func findDoctorDefinition(path, workspaceID string) (workspace.LocatedDefinition, error) {
	located, err := findLocatedDefinition(path, workspaceID)
	if err == nil {
		return located, nil
	}
	if err.Error() != "workspace not found" {
		return workspace.LocatedDefinition{}, err
	}

	items, listErr := listDefinitions(path)
	if listErr != nil {
		return workspace.LocatedDefinition{}, err
	}

	query := strings.ToLower(workspaceID)
	var matches []string
	for _, item := range items {
		id := item.Definition.ID
		if strings.Contains(strings.ToLower(id), query) {
			matches = append(matches, id)
		}
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return workspace.LocatedDefinition{}, err
	}
	return workspace.LocatedDefinition{}, fmt.Errorf("workspace not found; did you mean: %s", strings.Join(matches, ", "))
}

func runDoctorCommand(cmd *cobra.Command, located workspace.LocatedDefinition) error {
	report := diagnoseWorkspace(cmd, located, cmd.OutOrStdout())
	return writeDoctorReport(cmd.OutOrStdout(), report)
}

func diagnoseWorkspace(cmd *cobra.Command, located workspace.LocatedDefinition, stdout io.Writer) doctor.Report {
	runner := newLifecycleRunner(stdout, cmd.ErrOrStderr())
	return doctor.Diagnose(located, runner)
}

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeDoctorReport(out io.Writer, report doctor.Report) error {
	_, err := fmt.Fprintf(out, "Workspace: %s\nState: %s\n\nChecks:\n", report.WorkspaceID, report.State)
	if err != nil {
		return err
	}

	for _, check := range report.Checks {
		symbol := checkStatusSymbol(check.Status)
		if _, err := fmt.Fprintf(out, "  %s %s: %s\n", symbol, check.Name, check.Detail); err != nil {
			return err
		}
	}

	_, err = fmt.Fprintln(out, "\nNext:")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(out, "  %s\n", report.NextAction)
	return err
}

func checkStatusSymbol(status doctor.CheckStatus) string {
	switch status {
	case doctor.CheckPass:
		return "✓"
	case doctor.CheckFail:
		return "✗"
	case doctor.CheckWarn:
		return "!"
	case doctor.CheckSkip:
		return "-"
	default:
		return "-"
	}
}
