package cmd

import (
	"github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/netlify/netlify-commerce/conf"
	"github.com/netlify/netlify-commerce/models"
)

// rootCmd will run the log streamer
var rootCmd = cobra.Command{
	Use:  "netlify-commerce",
	Long: "A service that will validate restful transactions and send them to stripe.",
	Run: func(cmd *cobra.Command, args []string) {
		execWithConfig(cmd, serve)
	},
}

// NewRoot will add flags and subcommands to the different commands
func RootCmd() *cobra.Command {
	rootCmd.PersistentFlags().StringP("config", "c", "", "The configuration file")
	rootCmd.AddCommand(&serveCmd, &migrateCmd, &versionCmd)
	return &rootCmd
}

func execWithConfig(cmd *cobra.Command, fn func(config *conf.Configuration)) {
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		logrus.Fatalf("%+v", err)
	}

	config, err := conf.Load(configFile)
	if err != nil {
		logrus.Fatalf("Failed to load configration: %+v", err)
	}

	if config.DB.Namespace != "" {
		models.Namespace = config.DB.Namespace
	}

	fn(config)
}
