package models

import (
	// this is where we do the connections
	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/mysql"
	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/postgres"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/jinzhu/gorm"
	"github.com/netlify/gocommerce/conf"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Namespace puts all tables names under a common
// namespace. This is useful if you want to use
// the same database for several services and don't
// want table names to collide.
var Namespace string

// Connect will connect to that storage engine
func Connect(config *conf.GlobalConfiguration) (*gorm.DB, error) {
	if config.DB.Dialect == "" {
		config.DB.Dialect = config.DB.Driver
	}
	db, err := gorm.Open(config.DB.Dialect, config.DB.Driver, config.DB.URL)
	if err != nil {
		return nil, errors.Wrap(err, "opening database connection")
	}

	if logrus.StandardLogger().Level == logrus.DebugLevel {
		db.LogMode(true)
	}

	err = db.DB().Ping()
	if err != nil {
		return nil, errors.Wrap(err, "checking database connection")
	}

	if config.DB.Automigrate {
		if err := AutoMigrate(db); err != nil {
			return nil, errors.Wrap(err, "migrating tables")
		}
	}

	return db, nil
}

func tableName(defaultName string) string {
	if Namespace != "" {
		return Namespace + "_" + defaultName
	}
	return defaultName
}

// AutoMigrate runs the gorm automigration for all models
func AutoMigrate(db *gorm.DB) error {
	db = db.AutoMigrate(Address{},
		LineItem{},
		AddonItem{},
		PriceItem{},
		Hook{},
		Download{},
		Order{},
		OrderNote{},
		Transaction{},
		User{},
		Event{},
		Instance{},
	)
	return db.Error
}
