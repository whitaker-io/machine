// Copyright © 2020 Jonathan Whitaker <jonathan@whitaker.io>

package cmd

import (
	"fmt"
	"go/build"
	"log"
	"os"
	"path/filepath"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/whitaker-io/go-mix/lib/util"
	"github.com/whitaker-io/machine/cmd/templates"
)

var cfgFile string
var force bool

var rootCmd = &cobra.Command{
	Use:   "machine",
	Short: "",
	Long:  ``,
}

// Execute func for running the commands
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cmd.yaml)")
	rootCmd.PersistentFlags().BoolVar(&force, "force", false, "override files")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.SetConfigName(".cmd")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func addFN(project templates.Project) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		if err := templates.GenerateProject(getCurrentDir(), project, force, config(args)); err != nil {
			fmt.Printf("cannot create project - err: %v", err)
			os.Exit(1)
		}
	}
}

func getCurrentDir() string {
	ex, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}
	return ex
}

func config(args []string) map[string]interface{} {
	gosrc := os.Getenv("GOSRC")
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}

	if gosrc == "" {
		gosrc = filepath.Join(gopath, "src")
	}

	currentDir := util.GetCurrentDir()

	return map[string]interface{}{
		"Path": strings.TrimPrefix(currentDir, gosrc+"/"),
		"Name": args[0],
	}
}
