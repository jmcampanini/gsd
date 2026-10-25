package cli

import (
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/clierror"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

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
		return clierror.NormalizeApplication(err)
	}
	defer func() {
		_ = closer.Close()
	}()

	return clierror.NormalizeApplication(run(available))
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
	writeHuman func(render.HumanOutput, T) error,
) error {
	return withTaskApplication(command, options, factory, func(application task.Application) error {
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
	writeHuman func(render.HumanOutput, T) error,
) error {
	return withBoardApplication(command, options, factory, func(application board.Application) error {
		result, err := run(application)
		if err != nil {
			return err
		}
		return renderResult(command, options, result, jsonPayload, writeHuman)
	})
}
