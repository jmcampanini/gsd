package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

func newListCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	statusValue := string(task.ListStatusOpen)
	var due bool
	var overdue bool
	var deferred bool
	var projectIDValue string
	var areaIDValue string
	var tagValue string
	command := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Long: `List tasks with optional filters, ordered by position and then ID across
all containers.

` + clihelp.StatusFilterHelp + `

--project ID keeps tasks in that project, and --area ID keeps loose tasks
whose own area is ID, not tasks in the area's projects; the two cannot be
combined (exit 1), and an unknown project, area, or tag is an error
(exit 1). --tag NAME keeps tasks carrying that tag, matched
case-insensitively. --due keeps tasks with a due date, --overdue keeps
open tasks due before today, and --deferred keeps tasks whose defer date
is after today or whose stage defer has not been reached; those three
are mutually exclusive (usage error, exit 2). Filters combine with AND.

The human table has id, title, status, and dates columns, with title and
dates as in 'gsd inbox'; an empty result prints nothing. With --json an
array of task rows is written (fields as in 'gsd show --help'), [] when
empty.

` + clihelp.OutputContractHelp,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			status, err := task.ParseListStatus(statusValue)
			if err != nil {
				return err
			}
			projectID, err := parseProjectIDFlag(command, projectIDValue)
			if err != nil {
				return err
			}

			areaID, err := parseAreaIDFlag(command, areaIDValue)
			if err != nil {
				return err
			}

			selector := task.DateSelectorNone
			if due {
				selector = task.DateSelectorDue
			}
			if overdue {
				selector = task.DateSelectorOverdue
			}
			if deferred {
				selector = task.DateSelectorDeferred
			}
			listOptions := task.ListOptions{Status: status, Date: selector, ProjectID: projectID, AreaID: areaID}
			if command.Flags().Changed("tag") {
				listOptions.Tag = &tagValue
			}

			return withTaskApplication(command, options, factory, func(application task.Application) error {
				tasks, err := application.List(command.Context(), listOptions)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, tasks, render.HumanOutput.WriteTaskList)
			})
		},
	}
	command.Flags().StringVar(&statusValue, "status", statusValue, "open, done, cancelled, or all")
	command.Flags().StringVar(&projectIDValue, "project", "", "filter by project ID")
	command.Flags().StringVar(&areaIDValue, "area", "", "filter by area ID")
	command.Flags().StringVar(&tagValue, "tag", "", "filter by tag")
	command.Flags().BoolVar(&due, "due", false, "list tasks with due dates")
	command.Flags().BoolVar(&overdue, "overdue", false, "list overdue open tasks")
	command.Flags().BoolVar(&deferred, "deferred", false, "list tasks deferred beyond today")
	command.MarkFlagsMutuallyExclusive("due", "overdue", "deferred")

	return command
}
