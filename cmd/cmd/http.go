// Copyright © 2020 Jonathan Whitaker <jonathan@whitaker.io>

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/whitaker-io/machine/cmd/templates"
)

// httpCmd represents the http command
var httpCmd = &cobra.Command{
	Use:   "http <name>",
	Short: "Adds a new machine.Stream to the machine.Pipe",
	Long: `Adds a new machine.Stream to the machine.Pipe
	This command sets up an HTTP based Stream in the Pipe
	
	Example: machine http wehbook
	`,
	Run: addFN(templates.Project{
		Dirs: map[string]templates.Project{
			"pipe": {
				Files: map[string]string{
					"{{.Name | ToLower}}.go": httpFile,
				},
			},
		},
	}),
}

func init() {
	rootCmd.AddCommand(httpCmd)
}

const httpFile = `package pipe

import "github.com/whitaker-io/machine"

func init() {
	// HTTP Stream ready to be built
	Pipe.StreamHTTP("{{UUID}}",
		&machine.Option{FIFO: boolP(false)},
		&machine.Option{Injectable: boolP(true)},
		&machine.Option{Metrics: boolP(true)},
		&machine.Option{Span: boolP(true)},
		&machine.Option{BufferSize: intP(0)},
	)
}`
