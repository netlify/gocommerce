package models

type SiteSettings struct {
	Taxes []*Tax `json:"taxes"`
}

func (SiteSettings) TableName() string {
	return tableName("sites_settings")
}

type Tax struct {
	Percentage   uint64   `json:"percentage"`
	ProductTypes []string `json:"product_types"`
	Countries    []string `json:"countries"`
}
