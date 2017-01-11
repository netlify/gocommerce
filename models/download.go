package models

import "github.com/netlify/netlify-commerce/assetstores"

type Download struct {
	ID string `json:"id"`

	OrderID    string `json:"order_id"`
	LineItemID int64  `json:"line_item_id"`

	Title  string `json:"title"`
	Format string `json:"format"`
	URL    string `json:"url"`

	DownloadCount uint64 `json:"downloads"`
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
