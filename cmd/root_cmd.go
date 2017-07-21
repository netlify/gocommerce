package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
)

// rootCmd will run the log streamer
var rootCmd = cobra.Command{
	Use:  "gocommerce",
	Long: "A service that will validate restful transactions and send them to stripe.",
	Run:  serveCmdFunc,
}

// RootCmd will add flags and subcommands to the different commands
func RootCmd() *cobra.Command {
	rootCmd.PersistentFlags().StringP("config", "c", "", "The configuration file")
	rootCmd.AddCommand(&serveCmd, &migrateCmd, &versionCmd)
	return &rootCmd
}

func execWithConfig(cmd *cobra.Command, fn func(config *conf.GlobalConfiguration)) {
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		logrus.Fatalf("%+v", err)
	}

	config, err := conf.LoadGlobal(configFile)
	if err != nil {
		logrus.Fatalf("Failed to load configuration: %+v", err)
	}

	if config.DB.Namespace != "" {
		models.Namespace = config.DB.Namespace
	}

	fn(config)
}
