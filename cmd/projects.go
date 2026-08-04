package cmd

import (
	"context"
	"errors"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/spf13/cobra"
)

func newProjectsCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := &cobra.Command{
		Use:   "projects",
		Short: "Manage projects",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return errors.New("projects requires a subcommand")
		},
	}
	command.AddCommand(
		newProjectsAddCommand(options, factory),
		newProjectsListCommand(options, factory),
	)

	return command
}

func newProjectsAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var note string
	var areaIDValue string
	var tags []string
	command := &cobra.Command{
		Use:   "add TITLE",
		Short: "Add a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			areaID, err := parseAreaIDFlag(command, areaIDValue)
			if err != nil {
				return err
			}
			resolvedNote, err := resolveNote(command, note)
			if err != nil {
				return err
			}

			fields := project.AddFields{AreaID: areaID, Title: args[0], Note: resolvedNote, Tags: tags}
			return withProjectApplication(command, options, factory, func(application project.Application) error {
				created, addErr := application.Add(command.Context(), fields)
				if addErr != nil {
					return addErr
				}
				return writeCommandOutput(command, options, created, humanOutput.writeAddedProject)
			})
		},
	}
	command.Flags().StringVar(&note, "note", "", "project note or - to read stdin")
	command.Flags().StringVar(&areaIDValue, "area", "", "area ID")
	command.Flags().StringArrayVar(&tags, "tag", nil, "tag name to attach (repeatable)")

	return command
}

func newProjectsListCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	statusValue := string(project.ListStatusOpen)
	var areaIDValue string
	command := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			status, err := project.ParseListStatus(statusValue)
			if err != nil {
				return err
			}
			areaID, err := parseAreaIDFlag(command, areaIDValue)
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				projects, listErr := application.List(command.Context(), project.ListOptions{Status: status, AreaID: areaID})
				if listErr != nil {
					return listErr
				}
				return writeCommandOutput(command, options, projects, humanOutput.writeProjectList)
			})
		},
	}
	command.Flags().StringVar(
		&statusValue,
		"status",
		statusValue,
		"filter by status: open, done, cancelled, or all",
	)
	command.Flags().StringVar(&areaIDValue, "area", "", "filter by area ID")

	return command
}

func newProjectCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := &cobra.Command{
		Use:   "project",
		Short: "Manage a project",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return errors.New("project requires a subcommand")
		},
	}
	command.AddCommand(
		newProjectCancelCommand(options, factory),
		newProjectDeleteCommand(options, factory),
		newProjectDoneCommand(options, factory),
		newProjectEditCommand(options, factory),
		newProjectReopenCommand(options, factory),
		newProjectShowCommand(options, factory),
		newProjectTagCommand(options, factory),
		newProjectUntagCommand(options, factory),
	)

	return command
}

func newProjectShowCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				found, showErr := application.Show(command.Context(), id)
				if showErr != nil {
					return showErr
				}
				return writeCommandOutput(command, options, found, humanOutput.writeProject)
			})
		},
	}
}

type projectTaggingMutation func(context.Context, project.Application, int64, []string) (project.Tagging, error)

func newProjectTagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newProjectTaggingCommand(
		options,
		factory,
		"tag ID NAME...",
		"Tag a project",
		"Tagged",
		func(ctx context.Context, application project.Application, id int64, names []string) (project.Tagging, error) {
			return application.Tag(ctx, id, names)
		},
	)
}

func newProjectUntagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newProjectTaggingCommand(
		options,
		factory,
		"untag ID NAME...",
		"Untag a project",
		"Untagged",
		func(ctx context.Context, application project.Application, id int64, names []string) (project.Tagging, error) {
			return application.Untag(ctx, id, names)
		},
	)
}

func newProjectTaggingCommand(
	options *rootOptions,
	factory applicationFactory,
	use string,
	short string,
	action string,
	mutate projectTaggingMutation,
) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MinimumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				tagging, err := mutate(command.Context(), application, id, args[1:])
				if err != nil {
					return err
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), tagging.Project)
				}

				return options.presentation.output(command).writeProjectTagging(action, tagging)
			})
		},
	}
}

func newProjectDoneCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newProjectResolveCommand(
		options,
		factory,
		"done ID",
		"Complete a project",
		"Done",
		project.ExitDone,
	)
}

func newProjectCancelCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newProjectResolveCommand(
		options,
		factory,
		"cancel ID",
		"Cancel a project",
		"Cancelled",
		project.ExitCancelled,
	)
}

func newProjectResolveCommand(
	options *rootOptions,
	factory applicationFactory,
	use string,
	short string,
	action string,
	exit project.Exit,
) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				resolution, err := application.Resolve(command.Context(), id, exit)
				if err != nil {
					return err
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), resolution)
				}

				return options.presentation.output(command).writeProjectResolution(action, resolution)
			})
		},
	}
}

func newProjectReopenCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "reopen ID",
		Short: "Reopen a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				reopened, err := application.Reopen(command.Context(), id)
				if err != nil {
					return err
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), reopened)
				}

				return options.presentation.output(command).writeProjectMutation("Reopened", reopened)
			})
		},
	}
}

func newProjectDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var recursive bool
	command := &cobra.Command{
		Use:   "delete ID",
		Short: "Delete a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				deletion, err := application.Delete(command.Context(), id, recursive)
				if err != nil {
					if code, ok := apperr.CodeOf(err); ok && code == apperr.Conflict && !recursive {
						return apperr.New(
							apperr.Conflict,
							err.Error()+"; use --recursive to delete the project and its tasks",
							err,
						)
					}
					return err
				}
				if options.json {
					if recursive {
						return writeJSON(command.OutOrStdout(), deletion)
					}

					return writeJSON(command.OutOrStdout(), deletion.Project)
				}

				return options.presentation.output(command).writeProjectDeletion(deletion)
			})
		},
	}
	command.Flags().BoolVar(&recursive, "recursive", false, "delete contained tasks")

	return command
}

func newProjectEditCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var title string
	var note string
	var areaIDValue string
	var noArea bool
	command := &cobra.Command{
		Use:   "edit ID",
		Short: "Edit a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}
			areaID, err := parseAreaIDFlag(command, areaIDValue)
			if err != nil {
				return err
			}

			if !anyFlagChanged(command, "title", "note", "area", "no-area") {
				return apperr.New(
					apperr.InvalidArgument,
					"project edit requires --title, --note, --area, or --no-area",
					nil,
				)
			}

			fields := project.EditFields{}
			fields.Area.Set = areaID
			fields.Area.Clear = noArea
			if command.Flags().Changed("title") {
				fields.Title = &title
			}
			if command.Flags().Changed("note") {
				resolvedNote, resolveErr := resolveNote(command, note)
				if resolveErr != nil {
					return resolveErr
				}
				fields.Note = &resolvedNote
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				edited, editErr := application.Edit(command.Context(), id, fields)
				if editErr != nil {
					return editErr
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), edited)
				}

				return options.presentation.output(command).writeProjectMutation("Edited", edited)
			})
		},
	}
	command.Flags().StringVar(&title, "title", "", "project title")
	command.Flags().StringVar(&note, "note", "", "project note or - to read stdin")
	command.Flags().StringVar(&areaIDValue, "area", "", "area ID")
	command.Flags().BoolVar(&noArea, "no-area", false, "remove the project from its area")
	command.MarkFlagsMutuallyExclusive("area", "no-area")

	return command
}
