package cmd

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/logbook"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/store"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

var Version = "dev"

type rootOptions struct {
	databasePath string
	json         bool
}

type applications struct {
	tasks    task.Application
	projects project.Application
	logbook  logbook.Application
}

type applicationFactory func(context.Context, string) (applications, io.Closer, error)

func Execute() int {
	return execute(newRootCommand(), os.Args[1:])
}

func execute(root *cobra.Command, args []string) int {
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		jsonMode, _ := root.PersistentFlags().GetBool("json")
		_ = writeCommandError(root.ErrOrStderr(), jsonMode, err)
		return exitCodeForError(err)
	}

	return 0
}

func newRootCommand() *cobra.Command {
	return newRootCommandWithFactory(defaultApplicationFactory)
}

func newRootCommandWithFactory(factory applicationFactory) *cobra.Command {
	return newRootCommandWithFactoryAndLocation(factory, time.Local)
}

func newRootCommandWithFactoryAndLocation(
	factory applicationFactory,
	location *time.Location,
) *cobra.Command {
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
		newAvailableCommand(options, factory),
		newCancelCommand(options, factory),
		newDeleteCommand(options, factory),
		newDoneCommand(options, factory),
		newEditCommand(options, factory),
		newInboxCommand(options, factory),
		newListCommand(options, factory),
		newLogbookCommand(options, factory, location),
		newProjectCommand(options, factory),
		newProjectsCommand(options, factory),
		newReopenCommand(options, factory),
		newShowCommand(options, factory),
	)

	return root
}

func defaultApplicationFactory(
	ctx context.Context,
	requestedPath string,
) (applications, io.Closer, error) {
	path, err := store.ResolvePath(requestedPath)
	if err != nil {
		return applications{}, nil, err
	}

	database, err := store.Open(ctx, path)
	if err != nil {
		return applications{}, nil, err
	}

	return applications{
		tasks:    task.NewService(store.NewTasks(database)),
		projects: project.NewService(store.NewProjects(database)),
		logbook:  logbook.NewService(store.NewLogbook(database)),
	}, database, nil
}

func withApplications(
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(applications) error,
) error {
	available, closer, err := factory(command.Context(), options.databasePath)
	if err != nil {
		return normalizeApplicationError(err)
	}
	defer func() {
		_ = closer.Close()
	}()

	return normalizeApplicationError(run(available))
}

func withTaskApplication(
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(task.Application) error,
) error {
	return withApplications(command, options, factory, func(available applications) error {
		return run(available.tasks)
	})
}

func withProjectApplication(
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(project.Application) error,
) error {
	return withApplications(command, options, factory, func(available applications) error {
		return run(available.projects)
	})
}

func withLogbookApplication(
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(logbook.Application) error,
) error {
	return withApplications(command, options, factory, func(available applications) error {
		return run(available.logbook)
	})
}

func normalizeApplicationError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.CodeOf(err); ok {
		return err
	}

	return apperr.New(apperr.Internal, err.Error(), err)
}

func exitCodeForError(err error) int {
	if err == nil {
		return 0
	}
	if _, ok := apperr.CodeOf(err); ok {
		return 1
	}

	return 2
}
