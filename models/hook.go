package models

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	"github.com/pborman/uuid"
)

const MaxConcurrentHooks = 5
const MaxRetries = 5

type Hook struct {
	ID uint64

	Type string

	Done   bool
	Failed bool

	URL     string
	Payload string

	ResponseStatus  string
	ResponseHeaders string
	ResponseBody    string
	ErrorMessage    *string

	Tries int

	CreatedAt   time.Time
	RunAfter    *time.Time
	LockedAt    *time.Time
	LockedBy    *string
	CompletedAt *time.Time
}

func (Hook) TableName() string {
	return tableName("hooks")
}

func NewHook(hookType string, url string, payload interface{}) *Hook {
	json, _ := json.Marshal(payload)
	return &Hook{
		Type:    hookType,
		URL:     url,
		Payload: string(json),
	}
}

func (h *Hook) Trigger(log *logrus.Entry) (*http.Response, error) {
	log.Infof("Triggering hook %v: %v", h.ID, h.URL)
	h.Tries += 1
	body := bytes.NewBufferString(h.Payload)
	resp, err := http.Post(h.URL, "application/json", body)
	return resp, err
}

func (h *Hook) handleError(db *gorm.DB, log *logrus.Entry, resp *http.Response, err error) {
	if err != nil {
		errString := err.Error()
		h.ErrorMessage = &errString
	} else {
		h.ErrorMessage = nil
	}

	if resp != nil && resp.Body != nil {
		body, _ := ioutil.ReadAll(resp.Body)
		h.ResponseBody = string(body)
		h.ResponseStatus = resp.Status
		headers, _ := json.Marshal(resp.Header)
		h.ResponseHeaders = string(headers)
	}

	now := time.Now()
	if h.Tries >= MaxRetries {
		log.Errorf("Hook %v failed more than %v times. Giving up.", h.ID, MaxRetries)
		h.Failed = true
		h.Done = true
		h.CompletedAt = &now
	} else {
		runAfter := now.Add(time.Duration(h.Tries) * 30 * time.Second)
		h.RunAfter = &runAfter
		log.Errorf("Hook %v failed - retrying at %v", h.ID, runAfter)
	}
	db.Save(h)
}

func (h *Hook) handleSuccess(db *gorm.DB, log *logrus.Entry, resp *http.Response) {
	log.Infof("Hook %v triggered. %v", h.ID, resp.Status)
	now := time.Now()
	h.Done = true
	h.ErrorMessage = nil
	h.ResponseStatus = resp.Status
	headers, _ := json.Marshal(resp.Header)
	h.ResponseHeaders = string(headers)
	body, _ := ioutil.ReadAll(resp.Body)
	h.ResponseBody = string(body)
	h.CompletedAt = &now
	db.Save(h)
}

func RunHooks(db *gorm.DB, log *logrus.Entry) {
	go func() {
		id := uuid.NewRandom().String()
		sem := make(chan bool, MaxConcurrentHooks)
		table := Hook{}.TableName()
		for {
			hooks := []*Hook{}
			db.Table(table).
				Where("done = 0 AND (locked_at IS NULL OR locked_at < ?) AND (run_after IS NULL OR run_after < ?)", time.Now().Add(-5*time.Minute), time.Now()).
				Updates(map[string]interface{}{"locked_at": time.Now(), "locked_by": id})

			db.Where("locked_by = ?", id).Find(&hooks)

			for _, hook := range hooks {
				sem <- true
				go func(hook *Hook) {
					resp, err := hook.Trigger(log)
					hook.LockedAt = nil
					hook.LockedBy = nil
					if err != nil || !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
						hook.handleError(db, log, resp, err)
					} else {
						hook.handleSuccess(db, log, resp)
					}
					<-sem
				}(hook)
			}

			time.Sleep(5 * time.Second)
		}
	}()
}
