package conf

import (
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Configuration holds all the confiruation for authlify
type Configuration struct {
	SiteURL string `mapstructure:"site_url" json:"site_url"`

	JWT struct {
		Secret         string `mapstructure:"secret" json:"secret"`
		AdminGroupName string `mapstructure:"admin_group_name" json:"admin_group_name"`
	} `mapstructure:"jwt" json:"jwt"`

	DB struct {
		Driver      string `mapstructure:"driver" json:"driver"`
		ConnURL     string `mapstructure:"url" json:"url"`
		Namespace   string `mapstructure:"namespace" json:"namespace"`
		Automigrate bool   `mapstructure:"automigrate" json:"automigrate"`
	} `mapstructure:"db" json:"db"`

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
	viper.SetConfigType("json")

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

	if err := viper.ReadInConfig(); err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "reading configuration from files")
	}

	config := new(Configuration)
	if err := viper.Unmarshal(config); err != nil {
		return nil, errors.Wrap(err, "unmarshaling configuration")
	}

	config, err := populateConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "populate config")
	}

	return validateConfig(config)
}

func validateConfig(config *Configuration) (*Configuration, error) {
	if config.DB.ConnURL == "" && os.Getenv("DATABASE_URL") != "" {
		config.DB.ConnURL = os.Getenv("DATABASE_URL")
	}

	if config.DB.Driver == "" && config.DB.ConnURL != "" {
		u, err := url.Parse(config.DB.ConnURL)
		if err != nil {
			return nil, errors.Wrap(err, "parsing db connection url")
		}
		config.DB.Driver = u.Scheme
	}

	if config.API.Port == 0 && os.Getenv("PORT") != "" {
		port, err := strconv.Atoi(os.Getenv("PORT"))
		if err != nil {
			return nil, errors.Wrap(err, "formatting PORT into int")
		}

		config.API.Port = port
	}

	if config.API.Port == 0 && config.API.Host == "" {
		config.API.Port = 8080
	}

	return config, nil
}
