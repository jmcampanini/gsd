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
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/config"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/logbook"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/search"
	"github.com/jmcampanini/gsd/internal/store"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/jmcampanini/gsd/internal/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var Version = "dev"

type rootOptions struct {
	configPath   string
	json         bool
	color        colorMode
	presentation *presentation
}

type applications struct {
	tasks    task.Application
	projects project.Application
	areas    area.Application
	boards   board.Application
	tags     tag.Application
	logbook  logbook.Application
	search   search.Application
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
	return newRootCommandWithRuntimeDependencies(
		factory,
		loadConfiguration,
		location,
		defaultPresentationDependencies(),
	)
}

func newRootCommandWithRuntimeDependencies(
	factory applicationFactory,
	loadConfiguration configurationLoader,
	location *time.Location,
	presentationDependencies presentationDependencies,
) *cobra.Command {
	return newRootCommandWithCaptureRunner(
		factory,
		loadConfiguration,
		location,
		presentationDependencies,
		tui.RunCapture,
	)
}

func newRootCommandWithCaptureRunner(
	factory applicationFactory,
	loadConfiguration configurationLoader,
	location *time.Location,
	presentationDependencies presentationDependencies,
	runCapture captureRunner,
) *cobra.Command {
	options := &rootOptions{color: colorAuto}
	availablePresentation := &presentation{
		mode:         &options.color,
		dependencies: presentationDependencies,
		location:     location,
	}
	options.presentation = availablePresentation
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
	root.PersistentFlags().Var(
		colorValue{mode: &options.color},
		"color",
		"control color output: auto, always, or never",
	)
	root.AddCommand(
		newAddCommand(options, factory),
		newAreaCommand(options, factory),
		newAreasCommand(options, factory),
		newAvailableCommand(options, factory),
		newBoardCommand(options, factory),
		newBoardsCommand(options, factory),
		newCancelCommand(options, factory),
		newCaptureCommand(options, factory, runCapture),
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
		newReorderCommand(options, factory),
		newSearchCommand(options, factory),
		newShowCommand(options, factory),
		newStageCommand(options, factory),
		newStagesCommand(options, factory),
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
		boards:   board.NewService(store.NewBoards(database)),
		tags:     tag.NewService(store.NewTags(database)),
		logbook:  logbook.NewService(store.NewLogbook(database)),
		search:   search.NewService(store.NewSearch(database)),
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

func withBoardApplication(
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(board.Application) error,
) error {
	return withApplications(command, options, factory, func(available applications) error {
		return run(available.boards)
	})
}

// The with*Output wrappers run one application call and hand the result to
// renderResult with a per-command JSON payload selector. Commands whose JSON
// and human modes share one payload use writeCommandOutput instead.
func withTaskOutput[T any](
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(task.Application) (T, error),
	jsonPayload func(T) any,
	writeHuman func(humanOutput, T) error,
) error {
	return withTaskApplication(command, options, factory, func(application task.Application) error {
		result, err := run(application)
		if err != nil {
			return err
		}
		return renderResult(command, options, result, jsonPayload, writeHuman)
	})
}

func withProjectOutput[T any](
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(project.Application) (T, error),
	jsonPayload func(T) any,
	writeHuman func(humanOutput, T) error,
) error {
	return withProjectApplication(command, options, factory, func(application project.Application) error {
		result, err := run(application)
		if err != nil {
			return err
		}
		return renderResult(command, options, result, jsonPayload, writeHuman)
	})
}

func withAreaOutput[T any](
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(area.Application) (T, error),
	jsonPayload func(T) any,
	writeHuman func(humanOutput, T) error,
) error {
	return withAreaApplication(command, options, factory, func(application area.Application) error {
		result, err := run(application)
		if err != nil {
			return err
		}
		return renderResult(command, options, result, jsonPayload, writeHuman)
	})
}

func withBoardOutput[T any](
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(board.Application) (T, error),
	jsonPayload func(T) any,
	writeHuman func(humanOutput, T) error,
) error {
	return withBoardApplication(command, options, factory, func(application board.Application) error {
		result, err := run(application)
		if err != nil {
			return err
		}
		return renderResult(command, options, result, jsonPayload, writeHuman)
	})
}

func withTagOutput[T any](
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(tag.Application) (T, error),
	jsonPayload func(T) any,
	writeHuman func(humanOutput, T) error,
) error {
	return withTagApplication(command, options, factory, func(application tag.Application) error {
		result, err := run(application)
		if err != nil {
			return err
		}
		return renderResult(command, options, result, jsonPayload, writeHuman)
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

func withSearchApplication(
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(search.Application) error,
) error {
	return withApplications(command, options, factory, func(available applications) error {
		return run(available.search)
	})
}

func normalizeApplicationError(err error) error {
	if err == nil {
		return nil
	}
	if code, ok := apperr.CodeOf(err); ok {
		message := err.Error()
		guided := message
		var resolvedProjects *domain.ResolvedProjectsError
		if errors.As(err, &resolvedProjects) {
			guided = appendRecoveryGuidance(guided, "reopen", "project reopen", resolvedProjects.IDs)
		}
		var archivedAreas *domain.ArchivedAreasError
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

// usageError builds an error the root adapter maps to exit 2: exitCodeForError
// treats every uncoded error as usage because Cobra parse failures arrive
// uncoded. Application errors must pass through normalizeApplicationError.
func usageError(message string) error {
	return errors.New(message)
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
