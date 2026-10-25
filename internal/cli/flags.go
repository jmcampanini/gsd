package cli

import (
	"fmt"
	"io"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/clierror"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/spf13/cobra"
)

func rejectFalseBooleanFlags(command *cobra.Command, names ...string) error {
	for _, name := range names {
		if !command.Flags().Changed(name) {
			continue
		}
		value, err := command.Flags().GetBool(name)
		if err != nil {
			return clierror.Usage("read --" + name + ": " + err.Error())
		}
		if !value {
			return clierror.Usage("--" + name + " cannot be false")
		}
	}
	return nil
}

func anyFlagChanged(command *cobra.Command, names ...string) bool {
	for _, name := range names {
		if command.Flags().Changed(name) {
			return true
		}
	}

	return false
}

func parseProjectIDFlag(command *cobra.Command, value string) (*int64, error) {
	if !command.Flags().Changed("project") {
		return nil, nil
	}

	id, err := project.ParseID(value)
	if err != nil {
		return nil, err
	}

	return &id, nil
}

func parseAreaIDFlag(command *cobra.Command, value string) (*int64, error) {
	if !command.Flags().Changed("area") {
		return nil, nil
	}

	id, err := area.ParseID(value)
	if err != nil {
		return nil, err
	}

	return &id, nil
}

func resolveNote(command *cobra.Command, value string) (string, error) {
	if value != "-" {
		return value, nil
	}

	contents, err := io.ReadAll(command.InOrStdin())
	if err != nil {
		return "", apperr.New(
			apperr.Internal,
			fmt.Sprintf("read note: %v", err),
			err,
		)
	}

	return string(contents), nil
}
