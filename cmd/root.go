package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/config"
	"github.com/jmcampanini/gsd/internal/logbook"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/store"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var Version = "dev"

type rootOptions struct {
	configPath string
	json       bool
}

type applications struct {
	tasks    task.Application
	projects project.Application
	areas    area.Application
	tags     tag.Application
	logbook  logbook.Application
}

type applicationFactory func(
	context.Context,
	string,
	bool,
	*pflag.FlagSet,
) (applications, io.Closer, error)

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
	return newRootCommandWithDependencies(factory, config.Load, location)
}

func newRootCommandWithDependencies(
	factory applicationFactory,
	loadConfiguration configurationLoader,
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

	root.PersistentFlags().StringVar(&options.configPath, "config", "", "path to a TOML config file")
	if err := config.RegisterFlags(root.PersistentFlags()); err != nil {
		panic(fmt.Sprintf("register config flags: %v", err))
	}
	root.PersistentFlags().BoolVar(&options.json, "json", false, "emit JSON output")
	root.AddCommand(
		newAddCommand(options, factory),
		newAreaCommand(options, factory),
		newAreasCommand(options, factory),
		newAvailableCommand(options, factory),
		newCancelCommand(options, factory),
		newConfigCommand(options, loadConfiguration),
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
		newTagCommand(options, factory),
		newTagsCommand(options, factory),
		newUntagCommand(options, factory),
	)

	return root
}

func defaultApplicationFactory(
	ctx context.Context,
	configPath string,
	configPathExplicit bool,
	flags *pflag.FlagSet,
) (applications, io.Closer, error) {
	loaded, _, err := config.Load(configPath, configPathExplicit, flags)
	if err != nil {
		return applications{}, nil, err
	}

	database, err := store.Open(ctx, loaded.DBPath)
	if err != nil {
		return applications{}, nil, err
	}

	return applications{
		tasks:    task.NewService(store.NewTasks(database)),
		projects: project.NewService(store.NewProjects(database)),
		areas:    area.NewService(store.NewAreas(database)),
		tags:     tag.NewService(store.NewTags(database)),
		logbook:  logbook.NewService(store.NewLogbook(database)),
	}, database, nil
}

func withApplications(
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(applications) error,
) error {
	flags := command.Root().PersistentFlags()
	available, closer, err := factory(
		command.Context(),
		options.configPath,
		flags.Changed("config"),
		flags,
	)
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

func withAreaApplication(
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(area.Application) error,
) error {
	return withApplications(command, options, factory, func(available applications) error {
		return run(available.areas)
	})
}

func withTagApplication(
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(tag.Application) error,
) error {
	return withApplications(command, options, factory, func(available applications) error {
		return run(available.tags)
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
	if code, ok := apperr.CodeOf(err); ok {
		message := err.Error()
		guided := message
		var resolvedProjects *project.ResolvedProjectsError
		if errors.As(err, &resolvedProjects) {
			guided = appendRecoveryGuidance(guided, "reopen", "project reopen", resolvedProjects.IDs)
		}
		var archivedAreas *area.ArchivedAreasError
		if errors.As(err, &archivedAreas) {
			guided = appendRecoveryGuidance(guided, "unarchive", "area unarchive", archivedAreas.IDs)
		}
		if guided != message {
			return apperr.New(code, guided, err)
		}
		return err
	}

	return apperr.New(apperr.Internal, err.Error(), err)
}

func appendRecoveryGuidance(message, verb, command string, ids []int64) string {
	unique := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		unique[id] = struct{}{}
	}
	ordered := make([]int64, 0, len(unique))
	for id := range unique {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left] < ordered[right] })

	commands := make([]string, 0, len(ordered))
	for _, id := range ordered {
		commands = append(commands, fmt.Sprintf("gsd %s %d", command, id))
	}
	if len(commands) == 0 {
		return message
	}

	return message + "; " + verb + " first: " + strings.Join(commands, "; ")
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
