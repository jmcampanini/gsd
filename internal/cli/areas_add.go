package cli

import (
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newAreasAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var note string
	var tags []string
	command := &cobra.Command{
		Use:   "add TITLE",
		Short: "Add an area",
		Long: `Create an active area titled TITLE and print it. TITLE must not be
blank. The area is placed last among all areas.

` + clihelp.NoteFlagHelp + `

` + clihelp.TagFlagHelp + `

On success one line reads '+ Added area ID: TITLE' followed by any tags;
with --json the new area row is written (fields as in 'gsd area show
--help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedNote, err := resolveNote(command, note)
			if err != nil {
				return err
			}

			fields := area.AddFields{Title: args[0], Note: resolvedNote, Tags: tags}
			return withAreaApplication(command, options, factory, func(application area.Application) error {
				created, addErr := application.Add(command.Context(), fields)
				if addErr != nil {
					return addErr
				}
				return writeCommandOutput(command, options, created, render.HumanOutput.WriteAddedArea)
			})
		},
	}
	command.Flags().StringVar(&note, "note", "", "area note or - to read stdin")
	command.Flags().StringArrayVar(&tags, "tag", nil, "tag name to attach (repeatable)")

	return command
}
