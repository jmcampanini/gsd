package cli

import (
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newProjectEditCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var title string
	var note string
	var areaIDValue string
	var noArea bool
	var boardTitle string
	var noBoard bool
	command := &cobra.Command{
		Use:   "edit ID",
		Short: "Edit a project",
		Long: `Change one or more fields of a project. At least one of --title, --note,
--area, --no-area, --board, and --no-board is required; without one the
command fails with invalid_argument (exit 1) before the database is
opened.

` + clihelp.IdGrammarHelp + `

--title TEXT and --note replace the title and note. --area ID and
--no-area file the project under another area or under none; an archived
source or destination area is a conflict (exit 1), while a resolved
project may be refiled. --board NAME and --no-board put the project on a
board, in its first stage and last among the projects there, or take it
off its board; a project already on NAME stays where it is. A board move
refuses a resolved project or an archived area (conflict, exit 1), needs
a board with at least one stage, and clears every stage defer on the
project's tasks, listing each cleared task. --area with --no-area and
--board with --no-board are mutually exclusive, and --no-area and
--no-board cannot be given as false (usage errors, exit 2).

` + clihelp.NoteFlagHelp + `

` + clihelp.BlockerGuidanceHelp + `

On success one line reads '~ Edited: project ID  TITLE', or with --board
or --no-board '~ Edited: ◆ ID  TITLE → BOARD/STAGE' (or '→ (no board)'),
followed by a 'Cleared stage defer' line per cleared task. With --json
the updated project row is written, except that a command with --board
or --no-board writes {"project":ROW,"cleared_defers":[ROW...]}, with
cleared_defers empty when nothing was cleared.

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := rejectFalseBooleanFlags(command, "no-area", "no-board"); err != nil {
				return err
			}
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}
			areaID, err := parseAreaIDFlag(command, areaIDValue)
			if err != nil {
				return err
			}

			if !anyFlagChanged(command, "title", "note", "area", "no-area", "board", "no-board") {
				return apperr.New(
					apperr.InvalidArgument,
					"project edit requires --title, --note, --area, --no-area, --board, or --no-board",
					nil,
				)
			}

			fields := project.EditRequest{}
			fields.Area.Set = areaID
			fields.Area.Clear = noArea
			if command.Flags().Changed("board") {
				fields.Board.Set = &boardTitle
			}
			fields.Board.Clear = noBoard
			if command.Flags().Changed("title") {
				fields.Title = &title
			}
			if command.Flags().Changed("note") {
				resolvedNote, resolveErr := resolveNote(command, note)
				if resolveErr != nil {
					return resolveErr
				}
				fields.Note = &resolvedNote
			}

			boardChanged := command.Flags().Changed("board") || command.Flags().Changed("no-board")
			return withProjectOutput(command, options, factory,
				func(application project.Application) (project.Edition, error) {
					return application.Edit(command.Context(), id, fields)
				},
				func(edited project.Edition) any {
					if boardChanged {
						return edited
					}
					return edited.Project
				},
				func(output render.HumanOutput, edited project.Edition) error {
					if boardChanged {
						return output.WriteProjectBoardEdit(edited)
					}
					return output.WriteProjectMutation(render.VerbEdited, edited.Project)
				},
			)
		},
	}
	command.Flags().StringVar(&title, "title", "", "project title")
	command.Flags().StringVar(&note, "note", "", "project note or - to read stdin")
	command.Flags().StringVar(&areaIDValue, "area", "", "area ID")
	command.Flags().BoolVar(&noArea, "no-area", false, "remove the project from its area")
	command.Flags().StringVar(&boardTitle, "board", "", "board name")
	command.Flags().BoolVar(&noBoard, "no-board", false, "remove the project from its board")
	command.MarkFlagsMutuallyExclusive("area", "no-area")
	command.MarkFlagsMutuallyExclusive("board", "no-board")

	return command
}
