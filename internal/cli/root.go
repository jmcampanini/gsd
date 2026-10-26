package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/config"
	"github.com/jmcampanini/gsd/internal/logbook"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/search"
	"github.com/jmcampanini/gsd/internal/store"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/jmcampanini/gsd/internal/tui"
	"github.com/jmcampanini/gsd/internal/tui/navigator"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Version is injected by the build task.
var Version = "dev"

type rootOptions struct {
	configPath   string
	json         bool
	color        render.ColorMode
	presentation *render.Presentation
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

// Execute runs the command tree and returns the process exit status.
func Execute() int {
	return execute(newRootCommand(), os.Args[1:])
}

func execute(root *cobra.Command, args []string) int {
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		jsonMode, _ := root.PersistentFlags().GetBool("json")
		_ = render.WriteCommandError(root.ErrOrStderr(), jsonMode, err)
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
		render.DefaultPresentationDependencies(),
	)
}

func newRootCommandWithRuntimeDependencies(
	factory applicationFactory,
	loadConfiguration configurationLoader,
	location *time.Location,
	presentationDependencies render.PresentationDependencies,
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
	presentationDependencies render.PresentationDependencies,
	runners runners,
) *cobra.Command {
	options := &rootOptions{color: render.ColorAuto}
	availablePresentation := &render.Presentation{
		Mode:         &options.color,
		Dependencies: presentationDependencies,
		Location:     location,
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

` + clihelp.DatabaseDiscoveryHelp + `

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

` + clihelp.OutputContractHelp + `

Only capture and tui are interactive: they require a terminal on stdin and
stdout and are the only commands that read keys. No command runs another
program or uses the network.

Run 'gsd config --help' for configuration precedence and the file format,
'gsd help exit-codes' for exit-status meanings, and 'gsd <group> --help'
for what each entity group's subcommands do.`,
		Version:            Version,
		DisableSuggestions: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}

	// Cobra's Find strips flags from the root operands before execute
	// registers --help and --version, so an unregistered --help would swallow
	// the next flag and 'gsd --help --db PATH' would reject PATH as an
	// unknown command. Registering both now lets that pass see they take no
	// value while the root stays free of a validator, which keeps Cobra's
	// help-topic routing. Suggestions are off so an unknown command reports
	// only 'unknown command "x" for "gsd"', as the groups already do.
	root.InitDefaultHelpFlag()
	root.InitDefaultVersionFlag()

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
	loaded, err := config.Load(configPath, configPathExplicit, flags)
	if err != nil {
		return applications{}, nil, err
	}

	database, err := store.Open(ctx, loaded.Effective.DBPath)
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

func exitCodeForError(err error) int {
	if err == nil {
		return 0
	}
	if _, ok := apperr.CodeOf(err); ok {
		return 1
	}

	return 2
}
