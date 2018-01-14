package conf

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/netlify/netlify-commons/nconf"
)

// DBConfiguration holds all the database related configuration.
type DBConfiguration struct {
	Dialect     string
	Driver      string `required:"true"`
	URL         string `envconfig:"DATABASE_URL" required:"true"`
	Namespace   string
	Automigrate bool
}

// JWTConfiguration holds all the JWT related configuration.
type JWTConfiguration struct {
	Secret         string `json:"secret"`
	AdminGroupName string `json:"admin_group_name" split_words:"true"`
}

type SMTPConfiguration struct {
	Host       string `json:"host"`
	Port       int    `json:"port" default:"587"`
	User       string `json:"user"`
	Pass       string `json:"pass"`
	AdminEmail string `json:"admin_email" split_words:"true"`
}

// GlobalConfiguration holds all the global configuration for gocommerce
type GlobalConfiguration struct {
	API struct {
		Host     string
		Port     int `envconfig:"PORT" default:"8080"`
		Endpoint string
	}
	DB                DBConfiguration
	Logging           nconf.LoggingConfig `envconfig:"LOG"`
	OperatorToken     string              `split_words:"true"`
	MultiInstanceMode bool
	SMTP              SMTPConfiguration `json:"smtp"`
}

// EmailContentConfiguration holds the configuration for emails, both subjects and template URLs.
type EmailContentConfiguration struct {
	OrderConfirmation string `json:"order_confirmation" split_words:"true"`
	OrderReceived     string `json:"order_received" split_words:"true"`
}

// Configuration holds all the per-tenant configuration for gocommerce
type Configuration struct {
	SiteURL string           `json:"site_url" split_words:"true" required:"true"`
	JWT     JWTConfiguration `json:"jwt"`

	SMTP SMTPConfiguration `json:"smtp"`

	Mailer struct {
		Subjects  EmailContentConfiguration `json:"subjects"`
		Templates EmailContentConfiguration `json:"templates"`
	} `json:"mailer"`

	Payment struct {
		Stripe struct {
			Enabled   bool   `json:"enabled"`
			PublicKey string `json:"public_key" split_words:"true"`
			SecretKey string `json:"secret_key" split_words:"true"`
		} `json:"stripe"`
		PayPal struct {
			Enabled  bool   `json:"enabled"`
			ClientID string `json:"client_id" split_words:"true"`
			Secret   string `json:"secret"`
			Env      string `json:"env"`
		} `json:"paypal"`
	} `json:"payment"`

	Downloads struct {
		Provider     string `json:"provider"`
		NetlifyToken string `json:"netlify_token" split_words:"true"`
	} `json:"downloads"`

	Coupons struct {
		URL      string `json:"url"`
		User     string `json:"user"`
		Password string `json:"password"`
	} `json:"coupons"`

	Webhooks struct {
		Order   string `json:"order"`
		Payment string `json:"payment"`
		Update  string `json:"update"`
		Refund  string `json:"refund"`

		Secret string `json:"secret"`
	} `json:"webhooks"`
}

func (c *Configuration) SettingsURL() string {
	return c.SiteURL + "/gocommerce/settings.json"
}

func loadEnvironment(filename string) error {
	var err error
	if filename != "" {
		err = godotenv.Load(filename)
	} else {
		err = godotenv.Load()
		// handle if .env file does not exist, this is OK
		if os.IsNotExist(err) {
			return nil
		}
	}
	return err
}

// LoadGlobal will construct the core config from the file
func LoadGlobal(filename string) (*GlobalConfiguration, error) {
	if err := loadEnvironment(filename); err != nil {
		return nil, err
	}

	config := new(GlobalConfiguration)
	if err := envconfig.Process("gocommerce", config); err != nil {
		return nil, err
	}
	if _, err := nconf.ConfigureLogging(&config.Logging); err != nil {
		return nil, err
	}
	return config, nil
}

// LoadConfig loads the per-instance configuration from a file
func LoadConfig(filename string) (*Configuration, error) {
	if err := loadEnvironment(filename); err != nil {
		return nil, err
	}

	config := new(Configuration)
	if err := envconfig.Process("gocommerce", config); err != nil {
		return nil, err
	}
	config.ApplyDefaults()
	return config, nil
}

// ApplyDefaults sets defaults for a Configuration
func (config *Configuration) ApplyDefaults() {
	if config.JWT.AdminGroupName == "" {
		config.JWT.AdminGroupName = "admin"
	}
}
