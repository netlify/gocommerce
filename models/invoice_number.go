package models

import (
	"log"
	"strings"

	"github.com/jinzhu/gorm"
)

type InvoiceNumber struct {
	InstanceID string `gorm:"primary_key"`
	Number     int64
}

// TableName returns the database table name for the LineItem model.
func (InvoiceNumber) TableName() string {
	return tableName("invoice_numbers")
}

// NextInvoiceNumber updates and returns the next invoice number for the instance
func NextInvoiceNumber(tx *gorm.DB, instanceID string) (int64, error) {
	number := InvoiceNumber{}
	if instanceID == "" {
		instanceID = "global-instance"
	}

	if result := tx.Where(InvoiceNumber{InstanceID: instanceID}).Attrs(InvoiceNumber{Number: 0}).FirstOrCreate(&number); result.Error != nil {
		return 0, result.Error
	}

	numberTable := tx.NewScope(InvoiceNumber{}).QuotedTableName()
	if result := tx.Raw("select number from "+numberTable+" where instance_id = ? for update", instanceID).Scan(&number); result.Error != nil {
		if strings.Contains(result.Error.Error(), "syntax error") {
			log.Println("This DB driver doesn't support select for update, hoping for the best...")
		} else {
			return 0, result.Error
		}
	}
	if result := tx.Model(number).Update("number", gorm.Expr("number + 1")); result.Error != nil {
		return 0, result.Error
	}

	return number.Number + 1, nil
}
