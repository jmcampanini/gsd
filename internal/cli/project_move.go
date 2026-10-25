package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newProjectMoveCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := reorderFlags{}
	command := &cobra.Command{
		Use:   "move ID STAGE",
		Short: "Move a project on its board",
		Long: `Move a project to stage STAGE of the board it is on, placing it last in
that stage unless a placement flag says otherwise. STAGE matches
case-insensitively; a stage that is not on the project's board is
not_found (exit 1). A project that is not on a board is a conflict (exit
1), and a move to the current stage without a placement flag changes
nothing. Stage defers on the project's tasks are kept.

` + clihelp.IdGrammarHelp + `

Placement is optional here, and the siblings are the projects in STAGE.

` + clihelp.PlacementHelp + `

` + clihelp.BlockerGuidanceHelp + `

On success one line reads '~ Moved: ◆ ID  TITLE → STAGE'; with --json the
updated project row is written (fields as in 'gsd project show --help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			if err := flags.validate(command); err != nil {
				return err
			}
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}
			placement, err := flags.optionalPlacement(command, project.ParseID)
			if err != nil {
				return err
			}

			return withProjectOutput(command, options, factory,
				func(application project.Application) (project.Movement, error) {
					return application.Move(command.Context(), id, args[1], placement)
				},
				func(moved project.Movement) any { return moved.Project },
				render.HumanOutput.WriteProjectMovement,
			)
		},
	}
	flags.registerOptional(command, "project")
	return command
}
