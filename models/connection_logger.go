package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type DBLogger struct {
	logrus.FieldLogger
}

func NewDBLogger(log logrus.FieldLogger) *DBLogger {
	return &DBLogger{log}
}

func (dbl *DBLogger) Print(params ...interface{}) {
	if len(params) <= 1 {
		return
	}

	level := params[0]
	log := dbl.WithField("gorm_level", level)

	if entry, ok := dbl.FieldLogger.(*logrus.Entry); ok && entry.Logger.Level >= logrus.TraceLevel {
		log = log.WithField("gorm_source", params[1])
	}

	if level != "sql" {
		log.Debug(params[2:]...)
		return
	}

	dur := params[2].(time.Duration)
	sql := params[3].(string)
	sqlValues := params[4].([]interface{})
	rows := params[5].(int64)

	values := ""
	if valuesJSON, err := json.Marshal(sqlValues); err == nil {
		values = string(valuesJSON)
	} else {
		values = fmt.Sprintf("%+v", sqlValues)
	}

	log.
		WithField("dur_ns", dur.Nanoseconds()).
		WithField("dur", dur).
		WithField("sql", strings.ReplaceAll(sql, `"`, `'`)).
		WithField("values", strings.ReplaceAll(values, `"`, `'`)).
		WithField("rows", rows).
		Debug("sql query")
}
