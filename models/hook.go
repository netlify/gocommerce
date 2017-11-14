package models

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const maxConcurrentHooks = 5
const maxRetries = 5
const retryPeriod = 30 * time.Second
const signatureExpiration = 5 * time.Minute

// Hook represents a webhook.
type Hook struct {
	ID uint64

	UserID string

	Type string

	Done   bool
	Failed bool

	URL     string
	Payload string `sql:"type:text"`
	Secret  string

	ResponseStatus  string
	ResponseHeaders string  `sql:"type:text"`
	ResponseBody    string  `sql:"type:text"`
	ErrorMessage    *string `sql:"type:text"`

	Tries int

	CreatedAt   time.Time
	RunAfter    *time.Time
	LockedAt    *time.Time
	LockedBy    *string
	CompletedAt *time.Time
}

// TableName returns the database table name for the Hook model.
func (Hook) TableName() string {
	return tableName("hooks")
}

// NewHook creates a Hook model.
func NewHook(hookType, siteURL, hookURL, userID, secret string, payload interface{}) (*Hook, error) {
	fullHookURL, err := url.Parse(hookURL)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse Webhook URL")
	}

	if !fullHookURL.IsAbs() {
		fullSiteURL, err := url.Parse(siteURL)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse Site URL")
		}
		fullHookURL.Scheme = fullSiteURL.Scheme
		fullHookURL.Host = fullSiteURL.Host
		fullHookURL.User = fullSiteURL.User
	}

	json, _ := json.Marshal(payload)
	return &Hook{
		Type:    hookType,
		UserID:  userID,
		URL:     fullHookURL.String(),
		Secret:  secret,
		Payload: string(json),
	}, nil
}

// Trigger creates and executes the HTTP request for a Hook.
func (h *Hook) Trigger(client *http.Client, log *logrus.Entry) (*http.Response, error) {
	log.Infof("Triggering hook %v: %v", h.ID, h.URL)
	h.Tries++
	body := bytes.NewBufferString(h.Payload)
	req, err := http.NewRequest("POST", h.URL, body)
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		return nil, err
	}
	if h.Secret != "" {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": h.UserID,
			"exp": time.Now().Add(signatureExpiration).Unix(),
		})
		tokenString, err := token.SignedString([]byte(h.Secret))
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Commerce-Signature", tokenString)
	}
	return client.Do(req)
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
	if h.Tries >= maxRetries {
		log.Errorf("Hook %v failed more than %v times. %v. Giving up.", h.ID, maxRetries, err)
		h.Failed = true
		h.Done = true
		h.CompletedAt = &now
	} else {
		runAfter := now.Add(time.Duration(h.Tries) * retryPeriod)
		h.RunAfter = &runAfter
		log.Errorf("Hook %v failed %v - retrying at %v", h.ID, err, runAfter)
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

// RunHooks creates a goroutine that triggers stored webhooks every 5 seconds.
func RunHooks(db *gorm.DB, log *logrus.Entry) {
	go func() {
		id := uuid.NewRandom().String()
		sem := make(chan bool, maxConcurrentHooks)
		table := Hook{}.TableName()
		client := &http.Client{}
		for {
			hooks := []*Hook{}
			tx := db.Begin()
			now := time.Now()

			tx.Table(table).
				Where("done = ? AND (locked_at IS NULL OR locked_at < ?) AND (run_after IS NULL OR run_after < ?)", false, now.Add(-5*time.Minute), now).
				Updates(map[string]interface{}{"locked_at": now, "locked_by": id})

			tx.Where("locked_by = ?", id).Find(&hooks)
			if rsp := tx.Commit(); rsp.Error != nil {
				log.WithError(rsp.Error).Error("Error querying for hooks")
			}

			var wg sync.WaitGroup
			for _, hook := range hooks {
				sem <- true
				wg.Add(1)
				go func(hook *Hook) {
					defer wg.Done()
					resp, err := hook.Trigger(client, log)
					hook.LockedAt = nil
					hook.LockedBy = nil
					tx := db.Begin()
					if err != nil || !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
						hook.handleError(tx, log, resp, err)
					} else {
						hook.handleSuccess(tx, log, resp)
					}
					tx.Commit()
					<-sem
				}(hook)
			}

			wg.Wait()
			time.Sleep(5 * time.Second)
		}
	}()
}
