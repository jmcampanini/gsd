package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newProjectsListCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	statusValue := string(project.ListStatusOpen)
	var areaIDValue string
	command := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Long: `List projects, ordered by position and then ID across all areas.

` + clihelp.StatusFilterHelp + `

--area ID keeps projects filed under that area; an unknown area is
not_found (exit 1).

The human table has id, title, and status columns; an empty result
prints nothing. With --json an array of project rows is written (fields
as in 'gsd project show --help'), [] when empty.

` + clihelp.OutputContractHelp,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			status, err := project.ParseListStatus(statusValue)
			if err != nil {
				return err
			}
			areaID, err := parseAreaIDFlag(command, areaIDValue)
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				projects, listErr := application.List(command.Context(), project.ListOptions{Status: status, AreaID: areaID})
				if listErr != nil {
					return listErr
				}
				return writeCommandOutput(command, options, projects, render.HumanOutput.WriteProjectList)
			})
		},
	}
	command.Flags().StringVar(
		&statusValue,
		"status",
		statusValue,
		"open, done, cancelled, or all",
	)
	command.Flags().StringVar(&areaIDValue, "area", "", "filter by area ID")

	return command
}
