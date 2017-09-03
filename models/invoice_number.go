package models

import (
	"errors"
	"log"
	"strings"

	"github.com/jinzhu/gorm"
)

type InvoiceNumber struct {
	ID     string
	Number int64
}

// TableName returns the database table name for the LineItem model.
func (InvoiceNumber) TableName() string {
	return tableName("invoice_numbers")
}

// NextInvoiceNumber updates and returns the next invoice number for the instance
func NextInvoiceNumber(tx *gorm.DB, instanceID string) (int64, error) {
	number := &InvoiceNumber{}

	if result := tx.FirstOrCreate(number, "id = ?", instanceID); result.Error != nil {
		return 0, result.Error
	}
	var numbers []int64

	if result := tx.Raw("select number from "+number.TableName()+" where id = ? for update", instanceID).Scan(&numbers); result.Error != nil {
		if strings.Contains(result.Error.Error(), "syntax error") {
			log.Println("This DB driver doesn't support select for update, hoping for the best...")
			numbers = append(numbers, number.Number)
		} else {
			return 0, result.Error
		}
	}
	if len(numbers) != 1 {
		return 0, errors.New("Error querying for next number")
	}
	if result := tx.Model(number).Update("number", gorm.Expr("number + 1")); result.Error != nil {
		return 0, result.Error
	}

	return numbers[0] + 1, nil
}
