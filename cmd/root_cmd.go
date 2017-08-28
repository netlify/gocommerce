package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
)

var configFile = ""

// rootCmd will run the log streamer
var rootCmd = cobra.Command{
	Use:  "gocommerce",
	Long: "A service that will validate restful transactions and send them to stripe.",
	Run: func(cmd *cobra.Command, args []string) {
		execWithConfig(cmd, serve)
	},
}

// RootCmd will add flags and subcommands to the different commands
func RootCmd() *cobra.Command {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "The configuration file")
	rootCmd.AddCommand(&serveCmd, &migrateCmd, &multiCmd, &versionCmd)
	return &rootCmd
}

func execWithConfig(cmd *cobra.Command, fn func(globalConfig *conf.GlobalConfiguration, config *conf.Configuration)) {
	globalConfig, err := conf.LoadGlobal(configFile)
	if err != nil {
		logrus.Fatalf("Failed to load configuration: %+v", err)
	}
	config, err := conf.LoadConfig(configFile)
	if err != nil {
		logrus.Fatalf("Failed to load configuration: %+v", err)
	}

	if globalConfig.DB.Namespace != "" {
		models.Namespace = globalConfig.DB.Namespace
	}
	fn(globalConfig, config)
}
