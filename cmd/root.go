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
	"github.com/jmcampanini/gsd/internal/tui/navigator"
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

// commandSpec is the data by which the parallel task, project, and area
// command constructors differ: the usage line, both help texts, and the
// mutation verb used in human output.
type commandSpec struct {
	long  string
	short string
	use   string
	verb  mutationVerb
}

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
	return newRootCommandWithRunners(
		factory,
		loadConfiguration,
		location,
		presentationDependencies,
		runners{
			capture:   tui.RunCapture,
			navigator: navigator.Run,
		},
	)
}

type runners struct {
	capture   captureRunner
	navigator navigatorRunner
}

func newRootCommandWithRunners(
	factory applicationFactory,
	loadConfiguration configurationLoader,
	location *time.Location,
	presentationDependencies presentationDependencies,
	runners runners,
) *cobra.Command {
	options := &rootOptions{color: colorAuto}
	availablePresentation := &presentation{
		mode:         &options.color,
		dependencies: presentationDependencies,
		location:     location,
	}
	options.presentation = availablePresentation
	root := &cobra.Command{
		Use:   "gsd",
		Short: "Get shit done",
		Long: `Get shit done: a personal task manager kept in one SQLite database.

A task has a title, a note, optional due and defer dates, tags, and a
status of open, done, or cancelled. A project groups tasks and can sit on
a board; an area groups projects and loose tasks and can be archived; a
task belongs to one project, one area, or neither, which is the inbox. A
board is an ordered list of stages that projects move through, and a task
can defer until its project reaches a stage or promote its project to the
next stage when done. A tag is a case-insensitive label that can be
attached to tasks, projects, and areas. The logbook lists done and
cancelled tasks and projects.

Task commands sit at the top level: 'gsd add TITLE', 'gsd list', 'gsd
show ID', 'gsd done ID', and their siblings. The other entities pair a
plural group that adds and lists with a singular group that acts on one
existing row: 'gsd projects' and 'gsd project', 'gsd areas' and 'gsd
area', 'gsd boards' and 'gsd board', 'gsd stages' and 'gsd stage', with
'gsd tags' covering both. 'gsd search EXPR' and 'gsd logbook' read across
entities, and 'gsd tui' and 'gsd capture' are the interactive commands.
Tasks, projects, and areas are addressed by ID; boards, stages, and tags
by name.

` + databaseDiscoveryHelp + `

--json, --color, --config, and --db apply to every command. --json
switches a noninteractive command to machine-readable output and is
rejected by capture, config, and tui. --color MODE needs a value (auto,
always, or never; default auto) and governs human output on stdout only:
always forces color, never disables it, and auto disables it when stdout
is not a terminal, TERM is dumb, or NO_COLOR is set and nonempty, except
that an explicit --color auto ignores NO_COLOR. FORCE_COLOR, CLICOLOR, and
CLICOLOR_FORCE are ignored. When color is on and stdout is a terminal, the
terminal background is queried once to choose a light or dark palette,
falling back to dark. Errors on stderr are never colored, and control
characters in stored text are escaped before display.

` + outputContractHelp + `

Only capture and tui are interactive: they require a terminal on stdin and
stdout and are the only commands that read keys. No command runs another
program or uses the network.

Run 'gsd config --help' for configuration precedence and the file format,
'gsd help exit-codes' for exit-status meanings, and 'gsd <group> --help'
for what each entity group's subcommands do.`,
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
		"color output: auto, always, or never",
	)
	root.AddCommand(
		newAddCommand(options, factory),
		newAreaCommand(options, factory),
		newAreasCommand(options, factory),
		newAvailableCommand(options, factory),
		newBoardCommand(options, factory),
		newBoardsCommand(options, factory),
		newCancelCommand(options, factory),
		newCaptureCommand(options, factory, runners.capture),
		newConfigCommand(options, loadConfiguration),
		newDeleteCommand(options, factory),
		newDoneCommand(options, factory),
		newEditCommand(options, factory),
		exitCodesTopic(),
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
		newTUICommand(options, factory, runners.navigator, location),
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
