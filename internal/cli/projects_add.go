package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newProjectsAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var note string
	var areaIDValue string
	var boardTitle string
	var tags []string
	command := &cobra.Command{
		Use:   "add TITLE",
		Short: "Add a project",
		Long: `Create an open project titled TITLE and print it. TITLE must not be
blank.

--area ID files the project under an area; an unknown area is not_found
and an archived one is a conflict (both exit 1). --board NAME puts the
project on a board, in its first stage and last among the projects
there; an unknown board is not_found and a board without stages is a
conflict (both exit 1). The project is placed last among the projects
filed under its area, or among those with no area.

` + clihelp.NoteFlagHelp + `

` + clihelp.TagFlagHelp + `

` + clihelp.BlockerGuidanceHelp + `

On success one line reads '+ Added project ID: TITLE' followed by any
tags; with --json the new project row is written (fields as in 'gsd
project show --help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			areaID, err := parseAreaIDFlag(command, areaIDValue)
			if err != nil {
				return err
			}
			resolvedNote, err := resolveNote(command, note)
			if err != nil {
				return err
			}

			fields := project.AddRequest{AreaID: areaID, Title: args[0], Note: resolvedNote, Tags: tags}
			if command.Flags().Changed("board") {
				fields.Board = &boardTitle
			}
			return withProjectApplication(command, options, factory, func(application project.Application) error {
				created, addErr := application.Add(command.Context(), fields)
				if addErr != nil {
					return addErr
				}
				return writeCommandOutput(command, options, created, render.HumanOutput.WriteAddedProject)
			})
		},
	}
	command.Flags().StringVar(&note, "note", "", "project note or - to read stdin")
	command.Flags().StringVar(&areaIDValue, "area", "", "area ID")
	command.Flags().StringVar(&boardTitle, "board", "", "board name")
	command.Flags().StringArrayVar(&tags, "tag", nil, "tag name to attach (repeatable)")

	return command
}
