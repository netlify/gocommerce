package mailer

import "github.com/netlify/gocommerce/models"

type noopMailer struct{}

func newNoopMailer() Mailer {
	return &noopMailer{}
}

func (m *noopMailer) OrderConfirmationMail(transaction *models.Transaction) error {
	return nil
}
func (m *noopMailer) OrderReceivedMail(transaction *models.Transaction) error {
	return nil
}

func (m *noopMailer) OrderConfirmationMailBody(transaction *models.Transaction, templateURL string) (string, error) {
	return "Order Confirmed", nil
}
