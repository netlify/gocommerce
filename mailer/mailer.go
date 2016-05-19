package mailer

import "github.com/netlify/gocommerce/conf"

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
