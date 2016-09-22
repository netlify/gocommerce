package conf

import (
	"strings"

	"github.com/spf13/viper"
)

// Configuration holds all the confiruation for authlify
type Configuration struct {
	SiteURL string `mapstructure:"site_url" json:"site_url"`

	JWT struct {
		Secret string `mapstructure:"" json:"secret"`
	} `mapstructure:"jwt" json:"jwt"`

	DB struct {
		Driver  string `mapstructure:"driver" json:"driver"`
		ConnURL string `mapstructure:"url" json:"url"`
	}

	API struct {
		Host string `mapstructure:"host" json:"host"`
		Port int    `mapstructure:"port" json:"port"`
	} `mapstructure:"api" json:"api"`

	Mailer struct {
		Host           string `mapstructure:"host" json:"host"`
		Port           int    `mapstructure:"port" json:"port"`
		User           string `mapstructure:"user" json:"user"`
		Pass           string `mapstructure:"pass" json:"pass"`
		TemplateFolder string `mapstructure:"template_folder" json:"template_folder"`
		AdminEmail     string `mapstructure:"admin_email" json:"admin_email"`
		MailSubjects   struct {
			OrderConfirmationMail string `mapstructure:"confirmation" json:"confirmation"`
		} `mapstructure:"mail_subjects" json:"mail_subjects"`
	} `mapstructure:"mailer" json:"mailer"`

	Payment struct {
		Stripe struct {
			SecretKey string `mapstructure:"secret_key" json:"secret_key"`
		} `mapstructure:"stripe" json:"stripe"`
		Paypal struct {
		} `mapstructure:"paypal" json:"paypal"`
	} `mapstructure:"payment" json:"payment"`
}

// Load will construct the config from the file `config.json`
func Load(configFile string) (*Configuration, error) {
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("./")                       // ./config.[json | toml]
		viper.AddConfigPath("$HOME/.gocommerce/")       // ~/.gocommerce/config.[json | toml] // Keep the configuration backwards compatible
		viper.AddConfigPath("$HOME/.netlify-commerce/") // ~/.netlify-commerce/config.[json | toml]
	}

	viper.SetEnvPrefix("NETLIFY_COMMERCE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	config := new(Configuration)

	if err := viper.Unmarshal(config); err != nil {
		return nil, err
	}

	return handleNested(config), nil
}

// This is supppper sad. It is b/c the marshal function doesn't work on nested
// values. The overrides work, but the marshal won't discover them.
// see: https://github.com/spf13/viper/issues/190
func handleNested(config *Configuration) *Configuration {
	config.JWT.Secret = viper.GetString("jwt.secret")

	config.DB.Driver = viper.GetString("db.driver")
	config.DB.ConnURL = viper.GetString("db.url")

	config.API.Host = viper.GetString("api.host")
	config.API.Port = viper.GetInt("api.port")

	config.Mailer.Host = viper.GetString("mailer.host")
	config.Mailer.Port = viper.GetInt("mailer.port")
	config.Mailer.User = viper.GetString("mailer.user")
	config.Mailer.Pass = viper.GetString("mailer.pass")
	config.Mailer.TemplateFolder = viper.GetString("mailer.template_folder")
	config.Mailer.AdminEmail = viper.GetString("mailer.admin_email")
	// TODO mail subjects....

	config.Payment.Stripe.SecretKey = viper.GetString("payment.stripe.secret_key")
	// TODO paypal

	return config
}
