package cli

import (
	"github.com/jmcampanini/gsd/internal/clierror"
	"github.com/spf13/cobra"
)

func newBoardCommandGroup(use, short, long string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return clierror.Usage(use + " requires a subcommand")
		},
	}
}
