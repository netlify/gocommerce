package models

type SiteSettings struct {
	Taxes []Tax `json:"taxes"`
}

type Tax struct {
	Percentage   int      `json:"percentage"`
	ProductTypes []string `json:"product_types"`
	Countries    []string `json:"countries"`
}
