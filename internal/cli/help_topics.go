package cli

import "github.com/spf13/cobra"

// helpTopicAnnotation marks a command built by newHelpTopic so the help test
// can find every topic without naming each one.
const helpTopicAnnotation = "gsd.help-topic"

// newHelpTopic builds a command that does no work and prints its own help,
// so 'gsd NAME' and 'gsd help NAME' print the same text.
func newHelpTopic(use, short, long string) *cobra.Command {
	return &cobra.Command{
		Use:         use,
		Short:       short,
		Long:        long,
		Annotations: map[string]string{helpTopicAnnotation: "true"},
		Args:        cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
}
