package cli

import (
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newBoardsListCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List boards",
		Long: `List every board, ordered by position and then ID, with its stages in
order.

The human table has board and stages columns, the stages joined by →; an
empty result prints nothing. With --json an array is written, each
element a board row (fields as in 'gsd board show --help') plus
"stages", an array of stage rows in order; [] when empty.

` + clihelp.OutputContractHelp,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				listed, err := application.List(command.Context())
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, listed, render.HumanOutput.WriteBoardList)
			})
		},
	}
}
