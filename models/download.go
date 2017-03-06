package models

import (
	"time"

	"github.com/netlify/gocommerce/assetstores"
)

type Download struct {
	ID string `json:"id"`

	OrderID    string `json:"order_id"`
	LineItemID int64  `json:"line_item_id"`

	Title  string `json:"title"`
	Sku    string `json:"sku"`
	Format string `json:"format"`
	URL    string `json:"url"`

	DownloadCount uint64 `json:"downloads"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-",sql:"index"`
}

func (Download) TableName() string {
	return tableName("downloads")
}

func (d *Download) SignURL(store assetstores.Store) error {
	signedURL, err := store.SignURL(d.URL)
	if err != nil {
		return err
	}
	d.URL = signedURL

	return nil
}
