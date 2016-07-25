package mailer

import (
	"fmt"

	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/models"
	"gopkg.in/gomail.v2"
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
	}
}

func price(amount uint64) string {
	return fmt.Sprintf("$%.2f", float64(amount)/100)
}

// OrderConfirmationMail sends a signup confirmation mail to a new user
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
	dial := gomail.NewDialer(m.Host, m.Port, m.User, m.Pass)
	return dial.DialAndSend(mail)
}
