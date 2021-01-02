// Copyright © 2021 Jonathan Whitaker <jonathan@whitaker.io>

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/whitaker-io/machine"
)

const (
	machinePortKey        = "machine.port"
	machineGracePeriodKey = "machine.grace_period"
	fiberConfigKey        = "fiber.config"
	serializationKey      = "machine.streams"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "serve - command starts a machine instance based on the config in $HOME/.machine.yaml",
	Long: `serve - command starts a machine instance based on the config in $HOME/.machine.yaml
	
	The following keys are read from $HOME/.machine.yaml
	EXAMPLE:

	fiber:
		config: # https://godoc.org/github.com/gofiber/fiber#Config
	machine:
		port: 5000 # int port value
		grace_period: 10 # int number of seconds to allow for graceful shutdown
		serializations: # list of serializations https://godoc.org/github.com/whitaker-io/machine#Serialization
	`,
	Run: func(cmd *cobra.Command, args []string) {
		fiberConfig := &fiber.Config{}

		if err := viper.UnmarshalKey(fiberConfigKey, fiberConfig); err != nil {
			fmt.Printf("error unmarshalling fiber config [%v]\n", err)
			os.Exit(1)
		}

		serializations := []*machine.Serialization{}

		if err := viper.UnmarshalKey(serializationKey, &serializations); err != nil {
			fmt.Printf("error unmarshalling serializations [%v]\n", err)
			os.Exit(1)
		}

		if len(serializations) < 1 {
			fmt.Printf("no serializations found\n")
			os.Exit(1)
		}

		pipe := machine.NewPipe(uuid.New().String(), logrus.New(), nil, *fiberConfig)

		for _, serialization := range serializations {
			if err := pipe.Load(serialization); err != nil {
				fmt.Printf("error loading serialization with id %s [%v]\n", serialization.ID, err)
				os.Exit(1)
			}
		}

		port := viper.GetInt(machinePortKey)
		gracePeriod := viper.GetInt64(machineGracePeriodKey)

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt)
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(gracePeriod)*time.Second)
		defer cancel()

		if err := pipe.Run(ctx, ":"+strconv.Itoa(port), time.Duration(gracePeriod)*time.Second); err != nil {
			fmt.Printf("error running pipe [%v]\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
