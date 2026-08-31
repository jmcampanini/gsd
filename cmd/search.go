package cmd

import (
	"github.com/jmcampanini/gsd/internal/search"
	"github.com/spf13/cobra"
)

func newSearchCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var related bool
	command := &cobra.Command{
		Use:   "search EXPR",
		Short: "Search tasks, projects, and areas",
		Long: `Search the titles, tags, and notes of tasks, projects, and areas with a
SQLite FTS5 query and list the matches by relevance.

EXPR is one operand in FTS5 syntax: words match whole tokens
case-insensitively and all of them must match, "quoted words" match a
phrase, word* matches a prefix, and AND, OR, NOT, and NEAR(...) combine
terms. A blank EXPR is invalid_argument, and an expression FTS5 cannot
parse is invalid_argument carrying the parser's message (both exit 1).

By default only a row's own title, tags, and note are searched.
--related also searches the context a row inherits, the title and tags
of a task's project and of the governing area, and lists direct matches
before context-only matches. Within a tier, hits rank by relevance with
title weighted above tags, tags above note, and note above context, then
tasks before projects before areas, then by ID.

The human table has kind, id, title, status, and context columns, where
context joins the project and governing area titles; an empty result
prints nothing. With --json an array of hits is written, each the
matched row's fields (as in that entity's show help) plus "kind" (task,
project, or area); [] when empty.

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return withSearchApplication(command, options, factory, func(application search.Application) error {
				hits, err := application.Search(command.Context(), args[0], related)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, hits, humanOutput.writeSearchHits)
			})
		},
	}
	command.Flags().BoolVar(&related, "related", false, "include matches from project and area context")
	return command
}
