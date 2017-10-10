package models

import (
	"fmt"
	"reflect"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

// cm should be pointer to a slice, e.g. &[]User{}
func cascadeDelete(tx *gorm.DB, query string, id interface{}, name string, cm interface{}) error {
	if result := tx.Where(query, id).Find(cm); result.Error != nil {
		return errors.Wrap(result.Error, fmt.Sprintf("Error deleting %s records", name))
	}

	// get direct reference to slice for loop
	t := reflect.Indirect(reflect.ValueOf(cm))
	for i := 0; i < t.Len(); i++ {
		// get pointer to model
		o := t.Index(i).Addr().Interface()
		if result := tx.Delete(o); result.Error != nil {
			return errors.Wrapf(result.Error, "Error deleting %s", name)
		}
	}
	return nil
}
