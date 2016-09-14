package models

import (
	// this is where we do the connections
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/netlify/gocommerce/conf"

	"github.com/jinzhu/gorm"
)

// Connect will connect to that storage engine
func Connect(config *conf.Configuration) (*gorm.DB, error) {
	db, err := gorm.Open(config.DB.Driver, config.DB.ConnURL)
	if err != nil {
		return nil, err
	}

	err = db.DB().Ping()
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(&Order{}, &Data{}, &Address{}, &LineItem{}, &OrderNote{}, &User{}, &Transaction{})

	return db, nil
}
