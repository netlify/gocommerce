package models

type SiteSettings struct {
	Taxes []*Tax `json:"taxes"`
}

type Tax struct {
	Percentage   uint64   `json:"percentage"`
	ProductTypes []string `json:"product_types"`
	Countries    []string `json:"countries"`
}
