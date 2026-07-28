package cmd

import (
	"context"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/jmcampanini/gsd/internal/store"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

var Version = "dev"

type rootOptions struct {
	databasePath string
	json         bool
}

type applicationFactory func(context.Context, string) (task.Application, io.Closer, error)

func Execute() int {
	return execute(newRootCommand(), os.Args[1:])
}

func execute(root *cobra.Command, args []string) int {
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		_ = writeCommandError(root.ErrOrStderr(), jsonModeForError(root, args, err), err)
		return exitCodeForError(err)
	}

	return 0
}

func newRootCommand() *cobra.Command {
	return newRootCommandWithFactory(defaultApplicationFactory)
}

func newRootCommandWithFactory(factory applicationFactory) *cobra.Command {
	options := &rootOptions{}
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

	root.PersistentFlags().StringVar(&options.databasePath, "db", "", "path to the SQLite database")
	root.PersistentFlags().BoolVar(&options.json, "json", false, "emit JSON output")
	root.AddCommand(
		newAddCommand(options, factory),
		newCancelCommand(options, factory),
		newDeleteCommand(options, factory),
		newDoneCommand(options, factory),
		newInboxCommand(options, factory),
		newListCommand(options, factory),
		newReopenCommand(options, factory),
		newShowCommand(options, factory),
	)

	return root
}

func defaultApplicationFactory(
	ctx context.Context,
	requestedPath string,
) (task.Application, io.Closer, error) {
	path, err := store.ResolvePath(requestedPath)
	if err != nil {
		return nil, nil, err
	}

	database, err := store.Open(ctx, path)
	if err != nil {
		return nil, nil, err
	}

	return task.NewService(database), database, nil
}

func withApplication(
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(task.Application) error,
) error {
	application, closer, err := factory(command.Context(), options.databasePath)
	if err != nil {
		return normalizeApplicationError(err)
	}
	defer func() {
		_ = closer.Close()
	}()

	return normalizeApplicationError(run(application))
}

func normalizeApplicationError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := task.ErrorCodeOf(err); ok {
		return err
	}

	return task.NewError(task.ErrorInternal, "internal error", err)
}

func exitCodeForError(err error) int {
	if err == nil {
		return 0
	}
	if code, ok := task.ErrorCodeOf(err); ok && code != task.ErrorUsage {
		return 1
	}

	return 2
}

func jsonModeForError(root *cobra.Command, args []string, err error) bool {
	if _, applicationError := task.ErrorCodeOf(err); applicationError {
		enabled, _ := root.PersistentFlags().GetBool("json")
		return enabled
	}

	return jsonModeRequested(args)
}

func jsonModeRequested(args []string) bool {
	enabled := false
	skipValue := false
	for _, argument := range args {
		if skipValue {
			skipValue = false
			continue
		}
		if argument == "--" {
			break
		}
		if argument == "--db" || argument == "--note" || argument == "--status" {
			skipValue = true
			continue
		}
		if argument == "--json" {
			enabled = true
			continue
		}
		value, found := strings.CutPrefix(argument, "--json=")
		if !found {
			continue
		}
		parsed, parseErr := strconv.ParseBool(value)
		if parseErr == nil {
			enabled = parsed
		}
	}

	return enabled
}
