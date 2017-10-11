package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/netlify/gocommerce/conf"
	"github.com/pkg/errors"
)

const baseConfigKey = ""

type Instance struct {
	ID string `json:"id"`
	// Netlify UUID
	UUID string `json:"uuid,omitempty"`

	RawBaseConfig string              `json:"-" sql:"type:text"`
	BaseConfig    *conf.Configuration `json:"config"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}

// TableName returns the table name used for the Instance model
func (i *Instance) TableName() string {
	return tableName("instances")
}

// AfterFind database callback.
func (i *Instance) AfterFind() error {
	if i.RawBaseConfig != "" {
		err := json.Unmarshal([]byte(i.RawBaseConfig), &i.BaseConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

// BeforeSave database callback.
func (i *Instance) BeforeSave() error {
	if i.BaseConfig != nil {
		data, err := json.Marshal(i.BaseConfig)
		if err != nil {
			return err
		}
		i.RawBaseConfig = string(data)
	}
	return nil
}

// Config loads the configuration and applies defaults.
func (i *Instance) Config() (*conf.Configuration, error) {
	if i.BaseConfig == nil {
		return nil, errors.New("no configuration data available")
	}

	baseConf := &conf.Configuration{}
	*baseConf = *i.BaseConfig
	baseConf.ApplyDefaults()

	return baseConf, nil
}

// GetInstance finds an instance by ID
func GetInstance(db *gorm.DB, instanceID string) (*Instance, error) {
	instance := Instance{}
	if rsp := db.Where("id = ?", instanceID).First(&instance); rsp.Error != nil {
		if rsp.RecordNotFound() {
			return nil, ModelNotFoundError{"instance"}
		}
		return nil, errors.Wrap(rsp.Error, "error finding instance")
	}
	return &instance, nil
}

func GetInstanceByUUID(db *gorm.DB, uuid string) (*Instance, error) {
	instance := Instance{}
	if rsp := db.Where("uuid = ?", uuid).First(&instance); rsp.Error != nil {
		if rsp.RecordNotFound() {
			return nil, ModelNotFoundError{"instance"}
		}
		return nil, errors.Wrap(rsp.Error, "error finding instance")
	}
	return &instance, nil
}

func CreateInstance(db *gorm.DB, instance *Instance) error {
	if result := db.Create(instance); result.Error != nil {
		return errors.Wrap(result.Error, "Error creating instance")
	}
	return nil
}

func UpdateInstance(db *gorm.DB, instance *Instance) error {
	if result := db.Save(instance); result.Error != nil {
		return errors.Wrap(result.Error, "Error updating instance record")
	}
	return nil
}

func DeleteInstance(db *gorm.DB, instance *Instance) error {
	return db.Delete(instance).Error
}

func (i *Instance) BeforeDelete(tx *gorm.DB) error {
	cascadeModels := map[string]interface{}{
		"order": &[]Order{},
		"user":  &[]User{},
	}
	for name, cm := range cascadeModels {
		if err := cascadeDelete(tx, "instance_id = ?", i.ID, name, cm); err != nil {
			return err
		}
	}

	delModels := map[string]interface{}{
		"transaction":    Transaction{},
		"invoice number": InvoiceNumber{},
	}

	for name, dm := range delModels {
		if result := tx.Delete(dm, "instance_id = ?", i.ID); result.Error != nil {
			return errors.Wrap(result.Error, fmt.Sprintf("Error deleting %s records", name))
		}
	}
	return nil
}
