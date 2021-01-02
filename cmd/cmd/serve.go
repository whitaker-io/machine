// Copyright © 2021 Jonathan Whitaker <jonathan@whitaker.io>

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"

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
	Short: "",
	Long:  ``,
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
		gracePeriod := viper.GetDuration(machineGracePeriodKey)

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt)
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), gracePeriod)
		defer cancel()

		if err := pipe.Run(ctx, ":"+strconv.Itoa(port), gracePeriod); err != nil {
			fmt.Printf("error running pipe [%v]\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
