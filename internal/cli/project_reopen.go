package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newProjectReopenCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "reopen ID",
		Short: "Reopen a project",
		Long: `Return a done or cancelled project to open, clearing done_at and
cancelled_at. Tasks cancelled when the project was resolved stay
cancelled; reopen them one at a time with 'gsd reopen ID'.

` + clihelp.IDGrammarHelp + `

A project that is already open is a conflict (exit 1), as is one filed
under an archived area.

` + clihelp.BlockerGuidanceHelp + `

On success one line reads '~ Reopened: project ID  TITLE'; with --json
the updated project row is written (fields as in 'gsd project show
--help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				reopened, err := application.Reopen(command.Context(), id)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, reopened, render.ProjectMutationWriter(render.VerbReopened))
			})
		},
	}
}
