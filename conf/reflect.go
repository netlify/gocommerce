package conf

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/spf13/viper"
)

const tagPrefix = "viper"

func populateConfig(config *Configuration) (*Configuration, error) {
	err := recursivelySet(reflect.ValueOf(config), "")
	if err != nil {
		return nil, err
	}

	return config, nil
}

func recursivelySet(val reflect.Value, prefix string) error {
	if val.Kind() != reflect.Ptr {
		return errors.New("Must pass a pointer")
	}

	// dereference
	val = reflect.Indirect(val)
	if val.Kind() != reflect.Struct {
		return errors.New("must be a reference to a struct")
	}

	// grab the type for this instance
	vType := reflect.TypeOf(val.Interface())

	// go through child fields
	for i := 0; i < val.NumField(); i++ {
		thisField := val.Field(i)
		thisType := vType.Field(i)
		tag := prefix + getTag(thisType)

		switch thisField.Kind() {
		case reflect.Struct:
			err := recursivelySet(thisField.Addr(), tag+".")
			if err != nil {
				return err
			}
		case reflect.Int:
			fallthrough
		case reflect.Int32:
			fallthrough
		case reflect.Int64:
			// you can only set with an int64 -> int
			configVal := int64(viper.GetInt(tag))
			thisField.SetInt(configVal)
		case reflect.Bool:
			configVal := viper.GetBool(tag)
			thisField.SetBool(configVal)
		case reflect.String:
			configVal := viper.GetString(tag)
			thisField.SetString(configVal)
		default:
			return fmt.Errorf("unexpected type detected ~ aborting: %s", thisField.Kind())
		}
	}

	return nil
}

func getTag(field reflect.StructField) string {
	// check if maybe we have a special magic tag
	tag := field.Tag
	if tag != "" {
		for _, prefix := range []string{tagPrefix, "mapstructure", "json"} {
			if v := tag.Get(prefix); v != "" {
				return v
			}
		}
	}

	return field.Name
}
