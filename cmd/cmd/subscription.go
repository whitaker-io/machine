// Copyright © 2020 Jonathan Whitaker <jonathan@whitaker.io>

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/whitaker-io/machine/cmd/templates"
)

// subscriptionCmd represents the subscription command
var subscriptionCmd = &cobra.Command{
	Use:   "subscription <name>",
	Short: "Adds a new machine.Stream to the machine.Pipe",
	Long: `Adds a new machine.Stream to the machine.Pipe
	This command sets up a Subscription Stream in the Pipe
	
	Example: machine subscription topicName
	`,
	Run: addFN(templates.Project{
		Dirs: map[string]templates.Project{
			"pipe": {
				Files: map[string]string{
					"{{.Name | ToLower}}.go": subscriptionFile,
				},
			},
		},
	}),
}

func init() {
	rootCmd.AddCommand(subscriptionCmd)
}

const subscriptionFile = `package pipe

import (
	"time"

	"github.com/whitaker-io/machine"
)

// Implement machine.Subscription
var {{.Name | ToLower}}Subscription machine.Subscription

func init() {
	// Subscription Stream ready to be built
	Pipe.StreamSubscription("{{UUID}}",
		{{.Name | ToLower}}Subscription,
		5 * time.Second,
		&machine.Option{FIFO: boolP(false)},
		&machine.Option{Injectable: boolP(true)},
		&machine.Option{Metrics: boolP(true)},
		&machine.Option{Span: boolP(true)},
		&machine.Option{BufferSize: intP(0)},
	)
}`
