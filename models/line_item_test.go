package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchingValuesCauseNoError(t *testing.T) {
	fakeOrder := NewOrder("sessionid", "bruce@wayne.com", "usd")
	li := testLineItem()
	meta := testMetadata()
	assert.Nil(t, li.Process(fakeOrder, meta))
}

func TestBadValuesCauseError(t *testing.T) {
	li := testLineItem()

	// mismatched sku
	badsku := testMetadata()
	badsku.Sku = "not-the-same"
	verifyError(t, li, badsku, "SKU")

	// mismatched VAT
	badvat := testMetadata()
	badvat.VAT = 456
	verifyError(t, li, badvat, "VAT")

	// unlisted currency
	badmoney := testMetadata()
	badmoney.Prices[0].Currency = "monopoly money"
	verifyError(t, li, badmoney, "usd")
}

func TestIfPriceIsNaN(t *testing.T) {
	li := testLineItem()
	badPrice := testMetadata()
	badPrice.Prices[0].Amount = "free"
	fakeOrder := NewOrder("sessionid", "bruce@wayne.com", "usd")

	err := li.Process(fakeOrder, badPrice)
	assert.NotNil(t, err)
}

func verifyError(t *testing.T, li *LineItem, meta *LineItemMetadata, sub string) {
	fakeOrder := NewOrder("sessionid", "bruce@wayne.com", "usd")
	err := li.Process(fakeOrder, meta).(FailedValidationError)
	assert.NotNil(t, err)
	if assert.Error(t, err) {
		assert.True(t, strings.Contains(err.Message, sub))
	}
}

func testMetadata() *LineItemMetadata {
	return &LineItemMetadata{
		Title:       "batmobile",
		Sku:         "na na na na",
		Description: "it has a rocket on the back",
		VAT:         23,
		Type:        "car",
		Prices: []PriceMetadata{PriceMetadata{
			Amount:   "1.00",
			Currency: "usd",
			VAT:      "23",
		}},
	}
}

func testLineItem() *LineItem {
	return &LineItem{
		ID:          123123,
		Title:       "batmobile",
		SKU:         "na na na na",
		Description: "it has a rocket on the back",
		VAT:         23,
		Type:        "car",
	}
}
