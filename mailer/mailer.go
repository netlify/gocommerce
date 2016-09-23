package mailer

import (
	"fmt"

	gomail "gopkg.in/gomail.v2"

	"github.com/netlify/netlify-commerce/conf"
	"github.com/netlify/netlify-commerce/models"
)

// Mailer will send mail and use templates from the site for easy mail styling
type Mailer struct {
	SiteURL        string
	TemplateFolder string
	Host           string
	Port           int
	User           string
	Pass           string
	AdminEmail     string
	MailSubjects   MailSubjects
}

// MailSubjects holds the subject lines for the emails
type MailSubjects struct {
	OrderConfirmationMail string
}

// NewMailer returns a new authlify mailer
func NewMailer(conf *conf.Configuration) *Mailer {
	mailConf := conf.Mailer
	return &Mailer{
		SiteURL:        conf.SiteURL,
		TemplateFolder: mailConf.TemplateFolder,
		Host:           mailConf.Host,
		Port:           mailConf.Port,
		User:           mailConf.User,
		Pass:           mailConf.Pass,
		AdminEmail:     mailConf.AdminEmail,
		MailSubjects: MailSubjects{
			OrderConfirmationMail: mailConf.MailSubjects.OrderConfirmationMail,
		},
	}
}

func price(amount uint64) string {
	return fmt.Sprintf("$%.2f", float64(amount)/100)
}

// OrderConfirmationMail sends an order confirmation to the user
func (m *Mailer) OrderConfirmationMail(transaction *models.Transaction) error {
	body := "<h2>Thank your for your order!</h2>\n<p><ul>"

	for _, item := range transaction.Order.LineItems {
		body += "<li>" + item.Title + " <strong>" + price(item.Price*item.Quantity) + "</strong></li>"
	}

	body += "</ul><p>Total amount: <strong>" + price(transaction.Order.Total) + "</strong></p>"

	mail := gomail.NewMessage()
	mail.SetHeader("From", m.AdminEmail)
	mail.SetHeader("To", transaction.Order.Email)
	mail.SetHeader("BCC", m.AdminEmail)
	mail.SetHeader("Subject", m.MailSubjects.OrderConfirmationMail)
	mail.SetBody("text/html", body)
	dial := gomail.NewPlainDialer(m.Host, m.Port, m.User, m.Pass)
	return dial.DialAndSend(mail)
}

// OrderReceivedMail sends a notification to the shop admin
func (m *Mailer) OrderReceivedMail(transaction *models.Transaction) error {
	body := "<h2>New Order from " + transaction.Order.Email + "</h2>\n<p><ul>"

	for _, item := range transaction.Order.LineItems {
		body += "<li>" + item.Title + " <strong>" + price(item.Price*item.Quantity) + "</strong></li>"
	}

	body += "</ul><p>Total amount: <strong>" + price(transaction.Order.Total) + "</strong></p>"

	mail := gomail.NewMessage()
	mail.SetHeader("From", m.AdminEmail)
	mail.SetHeader("To", m.AdminEmail)
	mail.SetHeader("BCC", m.AdminEmail)
	mail.SetHeader("Reply-To", transaction.Order.Email)
	mail.SetHeader("Subject", "New order from "+transaction.Order.Email)
	mail.SetBody("text/html", body)
	dial := gomail.NewPlainDialer(m.Host, m.Port, m.User, m.Pass)
	return dial.DialAndSend(mail)
}
