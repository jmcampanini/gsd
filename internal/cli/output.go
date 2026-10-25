package cli

import (
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

// renderResult is the single JSON/human dispatch point. Sites whose two modes
// share one payload route here through writeCommandOutput; commands whose JSON
// payload differs from the human model (flag-keyed envelopes, bare-row echoes)
// route through the with*Output wrappers in applications.go, which pair this dispatch
// with a per-command payload selector. capture and config are not dispatch
// sites: both reject --json outright. If another cross-cutting output surface
// lands (--quiet, CSV), extend renderResult - dispatch exists only here.
func renderResult[T any](
	command *cobra.Command,
	options *rootOptions,
	value T,
	jsonPayload func(T) any,
	writeHuman func(render.HumanOutput, T) error,
) error {
	if options.json {
		return render.WriteJSON(command.OutOrStdout(), jsonPayload(value))
	}
	return writeHuman(options.presentation.Output(command.InOrStdin(), command.OutOrStdout(), command.Root().PersistentFlags().Changed("color")), value)
}

func writeCommandOutput[T any](
	command *cobra.Command,
	options *rootOptions,
	value T,
	writeHuman func(render.HumanOutput, T) error,
) error {
	return renderResult(command, options, value, render.SamePayload[T], writeHuman)
}
