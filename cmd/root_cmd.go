package cmd

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/netlify/netlify-commerce/conf"
)

// RootCmd will run the log streamer
var RootCmd = cobra.Command{
	Use:  "netlify-commerce",
	Long: "A service that will validate restful transactions and send them to stripe.",
	Run: func(cmd *cobra.Command, args []string) {
		execWithConfig(cmd, serve)
	},
}

// InitCommands will add flags and subcommands to the different commands
func InitCommands() {
	RootCmd.PersistentFlags().StringP("config", "c", "", "The configuration file")
	RootCmd.AddCommand(&serveCmd, &migrateCmd)
}

func execWithConfig(cmd *cobra.Command, fn func(config *conf.Configuration)) {
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		log.Fatalf("%+v", err)
	}

	config, err := conf.Load(configFile)
	if err != nil {
		log.Fatalf("Failed to load configration: %+v", err)
	}

	fn(config)
}
