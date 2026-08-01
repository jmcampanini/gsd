package cmd

import (
	"errors"

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
	command := &cobra.Command{
		Use:   "add TITLE",
		Short: "Add a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedNote, err := resolveNote(command, note)
			if err != nil {
				return err
			}

			fields := project.AddFields{Title: args[0], Note: resolvedNote}
			return withProjectApplication(command, options, factory, func(application project.Application) error {
				created, addErr := application.Add(command.Context(), fields)
				if addErr != nil {
					return addErr
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), created)
				}

				return writeAddedProject(command.OutOrStdout(), created)
			})
		},
	}
	command.Flags().StringVar(&note, "note", "", "project note or - to read stdin")

	return command
}

func newProjectsListCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	statusValue := string(project.ListStatusOpen)
	command := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			status, err := project.ParseListStatus(statusValue)
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				projects, listErr := application.List(command.Context(), project.ListOptions{Status: status})
				if listErr != nil {
					return listErr
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), projects)
				}

				return writeProjectList(command.OutOrStdout(), projects)
			})
		},
	}
	command.Flags().StringVar(
		&statusValue,
		"status",
		statusValue,
		"filter by status: open, done, cancelled, or all",
	)

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
		newProjectEditCommand(options, factory),
		newProjectShowCommand(options, factory),
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
				if options.json {
					return writeJSON(command.OutOrStdout(), found)
				}

				return writeProject(command.OutOrStdout(), found)
			})
		},
	}
}

func newProjectEditCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var title string
	var note string
	command := &cobra.Command{
		Use:   "edit ID",
		Short: "Edit a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}

			fields := project.EditFields{}
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

				return writeProjectMutation(command.OutOrStdout(), "Edited", edited)
			})
		},
	}
	command.Flags().StringVar(&title, "title", "", "project title")
	command.Flags().StringVar(&note, "note", "", "project note or - to read stdin")

	return command
}
