package cli

import (
	"fmt"
	"io"
	"sort"

	"github.com/kenfdev/agw/internal/doctor"
	"github.com/kenfdev/agw/internal/workspace"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	var configPath string
	var all bool

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
				for _, located := range items {
					if err := runDoctorCommand(cmd, located); err != nil {
						return err
					}
				}
				return nil
			}
			located, err := findLocatedDefinition(configPath, args[0])
			if err != nil {
				return err
			}
			return runDoctorCommand(cmd, located)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config file path")
	cmd.Flags().BoolVar(&all, "all", false, "diagnose all known workspaces")
	return cmd
}

func runDoctorCommand(cmd *cobra.Command, located workspace.LocatedDefinition) error {
	runner := newLifecycleRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())
	report := doctor.Diagnose(located, runner)
	return writeDoctorReport(cmd.OutOrStdout(), report)
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
