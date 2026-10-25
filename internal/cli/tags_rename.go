package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/spf13/cobra"
)

func newTagsRenameCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "rename OLD NEW",
		Short: "Rename a tag",
		Long: `Rename tag OLD to NEW everywhere it is attached. NEW must not be blank,
and a different tag whose title matches NEW case-insensitively is a
conflict (exit 1); changing only the letter case of OLD is allowed.

` + clihelp.NameGrammarHelp + `

On success one line reads '~ Renamed tag OLD to NEW'; with --json the
updated tag row is written (id, title, created_at, updated_at).

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			return withTagOutput(command, options, factory,
				func(application tag.Application) (tag.Renaming, error) {
					return application.Rename(command.Context(), args[0], args[1])
				},
				func(renaming tag.Renaming) any { return renaming.Tag },
				func(output render.HumanOutput, renaming tag.Renaming) error {
					return output.WriteRenamedTag(renaming.PreviousTitle, renaming.Tag.Title)
				},
			)
		},
	}
}

func withTagOutput[T any](
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(tag.Application) (T, error),
	jsonPayload func(T) any,
	writeHuman func(render.HumanOutput, T) error,
) error {
	return withTagApplication(command, options, factory, func(application tag.Application) error {
		result, err := run(application)
		if err != nil {
			return err
		}
		return renderResult(command, options, result, jsonPayload, writeHuman)
	})
}
