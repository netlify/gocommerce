package cmd

import (
	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var migrateCmd = cobra.Command{
	Use:  "migrate",
	Long: "Migrate database strucutures. This will create new tables and add missing collumns and indexes.",
	Run: func(cmd *cobra.Command, args []string) {
		execWithConfig(cmd, migrate)
	},
}

func migrate(globalConfig *conf.GlobalConfiguration, config *conf.Configuration) {
	db, err := models.Connect(globalConfig)
	if err != nil {
		logrus.Fatalf("Error opening database: %+v", err)
	}
	defer db.Close()
}
