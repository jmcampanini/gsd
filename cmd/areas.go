package cmd

import (
	"errors"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/spf13/cobra"
)

func newAreasCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := &cobra.Command{
		Use:   "areas",
		Short: "Manage areas",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return errors.New("areas requires a subcommand")
		},
	}
	command.AddCommand(
		newAreasAddCommand(options, factory),
		newAreasListCommand(options, factory),
	)

	return command
}

func newAreasAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var note string
	command := &cobra.Command{
		Use:   "add TITLE",
		Short: "Add an area",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedNote, err := resolveNote(command, note)
			if err != nil {
				return err
			}

			fields := area.AddFields{Title: args[0], Note: resolvedNote}
			return withAreaApplication(command, options, factory, func(application area.Application) error {
				created, addErr := application.Add(command.Context(), fields)
				if addErr != nil {
					return addErr
				}
				return writeCommandOutput(command.OutOrStdout(), options.json, created, writeAddedArea)
			})
		},
	}
	command.Flags().StringVar(&note, "note", "", "area note or - to read stdin")

	return command
}

func newAreasListCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List areas",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return withAreaApplication(command, options, factory, func(application area.Application) error {
				areas, err := application.List(command.Context(), area.ListOptions{Slice: area.ListSliceActive})
				if err != nil {
					return err
				}
				return writeCommandOutput(command.OutOrStdout(), options.json, areas, writeAreaList)
			})
		},
	}
}

func newAreaCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := &cobra.Command{
		Use:   "area",
		Short: "Manage an area",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return errors.New("area requires a subcommand")
		},
	}
	command.AddCommand(
		newAreaEditCommand(options, factory),
		newAreaShowCommand(options, factory),
	)

	return command
}

func newAreaShowCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show an area",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := area.ParseID(args[0])
			if err != nil {
				return err
			}

			return withAreaApplication(command, options, factory, func(application area.Application) error {
				found, showErr := application.Show(command.Context(), id)
				if showErr != nil {
					return showErr
				}
				return writeCommandOutput(command.OutOrStdout(), options.json, found, writeArea)
			})
		},
	}
}

func newAreaEditCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var title string
	var note string
	command := &cobra.Command{
		Use:   "edit ID",
		Short: "Edit an area",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := area.ParseID(args[0])
			if err != nil {
				return err
			}

			if !anyFlagChanged(command, "title", "note") {
				return apperr.New(
					apperr.InvalidArgument,
					"area edit requires --title or --note",
					nil,
				)
			}

			fields := area.EditFields{}
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

			return withAreaApplication(command, options, factory, func(application area.Application) error {
				edited, editErr := application.Edit(command.Context(), id, fields)
				if editErr != nil {
					return editErr
				}
				return writeCommandOutput(command.OutOrStdout(), options.json, edited, writeEditedArea)
			})
		},
	}
	command.Flags().StringVar(&title, "title", "", "area title")
	command.Flags().StringVar(&note, "note", "", "area note or - to read stdin")

	return command
}
