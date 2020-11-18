// Copyright © 2020 Jonathan Whitaker <jonathan@whitaker.io>

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/whitaker-io/machine/cmd/templates"
)

// streamCmd represents the stream command
var streamCmd = &cobra.Command{
	Use:   "stream <name>",
	Short: "",
	Long:  ``,
	Run: addFN(templates.Project{
		Dirs: map[string]templates.Project{
			"pipe": {
				Files: map[string]string{
					"{{.Name | ToLower}}.go": streamFile,
				},
			},
		},
	}),
}

func init() {
	rootCmd.AddCommand(streamCmd)
}

const streamFile = `package pipe

import (
	"context"

	"github.com/whitaker-io/machine"
)

func init() {
	stream := machine.NewStream("{{UUID}}",
		func(c context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		},
		&machine.Option{FIFO: boolP(false)},
		&machine.Option{Injectable: boolP(true)},
		&machine.Option{Metrics: boolP(true)},
		&machine.Option{Span: boolP(false)},
		&machine.Option{BufferSize: intP(0)},
	)

	Pipe.Stream(stream)
}`
