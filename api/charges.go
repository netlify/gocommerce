package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/guregu/kami"
	"github.com/netlify/gocommerce/models"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"golang.org/x/net/context"
)

// MaxConcurrentLookups controls the number of simultaneous HTTP Order lookups
const MaxConcurrentLookups = 10

type ChargeParams struct {
	StripeToken string `json:"stripe_token"`
}

func (a *API) ChargeDo(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	params := &ChargeParams{}
	jsonDecoder := json.NewDecoder(r.Body)
	err := jsonDecoder.Decode(params)
	if err != nil {
		BadRequestError(w, fmt.Sprintf("Could not read params: %v", err))
		return
	}

	if params.StripeToken == "" {
		BadRequestError(w, "Payments requires a stripe_token")
		return
	}

	orderID := kami.Param(ctx, "order_id")
	tx := a.db.Begin()
	order := &models.Order{}

	if result := tx.Preload("LineItems").First(order, "id = ?", orderID); result.Error != nil {
		tx.Rollback()
		if result.RecordNotFound() {
			NotFoundError(w, "No order with this ID found")
		} else {
			InternalServerError(w, fmt.Sprintf("Error during database query: %v", result.Error))
		}
		return
	}

	if order.PaymentState == models.PaidState {
		tx.Rollback()
		BadRequestError(w, "This order has already been paid")
		return
	}

	token := getToken(ctx)
	if order.UserID == "" {
		if token != nil {
			id := token.Claims["id"].(string)
			order.UserID = id
			tx.Save(order)
		}
	} else {
		if token == nil {
			tx.Rollback()
			UnauthorizedError(w, "You must be logged in to pay for this order")
			return
		}
		id := token.Claims["id"].(string)
		if order.UserID != id {
			tx.Rollback()
			UnauthorizedError(w, "You must be logged in to pay for this order")
			return
		}
	}

	err = a.verifyLineItems(order.LineItems)
	if err != nil {
		tx.Rollback()
		InternalServerError(w, fmt.Sprintf("We failed to authorize the amount for this order: %v", err))
		return
	}

	ch, err := charge.New(&stripe.ChargeParams{
		Amount:   order.Total,
		Source:   &stripe.SourceParams{Token: params.StripeToken},
		Currency: "USD",
	})
	tr := &models.Transaction{
		OrderID:  order.ID,
		UserID:   order.UserID,
		Currency: "USD",
		Amount:   order.Total,
	}
	if ch != nil {
		tr.ProcessorID = ch.ID
	}
	if err != nil {
		tr.FailureCode = "500"
		tr.FailureDescription = err.Error()
		tr.Status = "failed"
	} else {
		tr.Status = "pending"
	}
	tx.Create(tr)

	if err != nil {
		tx.Commit()
		InternalServerError(w, fmt.Sprintf("There was an error charging your card: %v", err))
		return
	}

	order.PaymentState = models.PaidState
	tx.Save(order)
	tx.Commit()

	sendJSON(w, 200, tr)
}

func (a *API) verifyLineItems(items []models.LineItem) error {
	sem := make(chan int, MaxConcurrentLookups)
	var wg sync.WaitGroup
	sharedErr := verificationError{}
	for _, item := range items {
		sem <- 1
		wg.Add(1)
		go func(item *models.LineItem) {
			// Stop doing any work if there's already an error
			if sharedErr.err != nil {
				<-sem
				wg.Done()
				return
			}

			err := a.verifyLineItem(item)
			if err != nil {
				sharedErr.setError(err)
			}
			wg.Done()
			<-sem
		}(&item)
	}
	wg.Wait()

	return sharedErr.err
}

func (a *API) verifyLineItem(item *models.LineItem) error {
	fmt.Printf("Verifying line item %v", a.config.SiteURL+item.Path)
	doc, err := goquery.NewDocument(a.config.SiteURL + item.Path)
	if err != nil {
		return err
	}
	metaTag := doc.Find("head meta[name='product:price:amount']").First()
	if metaTag.Length() == 0 {
		return fmt.Errorf("No meta product tag found for '%v'", item.Title)
	}
	content, ok := metaTag.Attr("content")
	if !ok {
		return fmt.Errorf("No price set in meta tag for '%v'", item.Title)
	}
	amount, err := strconv.ParseUint(content, 10, 64)
	if err != nil {
		return fmt.Errorf("Malformed price set in meta tag for '%v'", item.Title)
	}
	if amount != item.Price {
		return fmt.Errorf(
			"Price for '%v' didn't match (%v on product page vs %v in order)",
			item.Title,
			amount,
			item.Price,
		)
	}

	return nil
}
