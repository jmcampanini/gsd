package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "dev"

type errorCategory uint8

const (
	errorCategoryNone errorCategory = iota
	errorCategoryApplication
	errorCategoryUsage
)

type categorizedError interface {
	error
	exitCategory() errorCategory
}

func Execute() int {
	root := newRootCommand()
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintf(root.ErrOrStderr(), "Error: %v\n", err)
		return exitCodeForError(err)
	}

	return 0
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "gsd",
		Short:         "Get shit done",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}

	return root
}

func exitCodeForError(err error) int {
	switch classifyError(err) {
	case errorCategoryNone:
		return 0
	case errorCategoryApplication:
		return 1
	case errorCategoryUsage:
		return 2
	default:
		return 2
	}
}

func classifyError(err error) errorCategory {
	if err == nil {
		return errorCategoryNone
	}

	var categorized categorizedError
	if errors.As(err, &categorized) && categorized.exitCategory() == errorCategoryApplication {
		return errorCategoryApplication
	}

	return errorCategoryUsage
}
