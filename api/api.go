package api

import (
	"net"
	"net/http"
	"regexp"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"github.com/netlify/gocommerce/conf"
	"github.com/netlify/gocommerce/mailer"
	"github.com/rs/cors"
	"github.com/rybit/kami"
	"github.com/satori/go.uuid"
)

var bearerRegexp = regexp.MustCompile(`^(?:B|b)earer (\S+$)`)

// API is the main REST API
type API struct {
	handler    http.Handler
	listener   net.Listener
	port       int
	db         *gorm.DB
	config     *conf.Configuration
	mailer     *mailer.Mailer
	httpClient *http.Client
	log        *logrus.Entry
}

// JWTClaims are what the token has access to
type JWTClaims struct {
	ID     string   `json:"id"`
	Email  string   `json:"email"`
	Groups []string `json:"groups"`
	*jwt.StandardClaims
}

// ListenAndServe starts the REST API
func (a *API) ListenAndServe(hostAndPort string) error {
	var err error
	a.listener, err = net.Listen("tcp", hostAndPort)
	if err != nil {
		return err
	}

	a.port = a.listener.Addr().(*net.TCPAddr).Port
	return http.Serve(a.listener, a.handler)
}

// Shutdown does what it sounds like
func (a *API) Shutdown() {
	a.listener.Close()
}

// NewAPI instantiates a new REST API
func NewAPI(config *conf.Configuration, db *gorm.DB, mailer *mailer.Mailer) *API {
	api := &API{
		config:     config,
		db:         db,
		mailer:     mailer,
		httpClient: &http.Client{},
		log:        logrus.NewEntry(logrus.StandardLogger()),
	}
	mux := kami.New()

	mux.Use("/", api.withToken)

	mux.Get("/", api.Index)
	mux.Get("/orders", api.trace(api.OrderList))
	mux.Post("/orders", api.OrderCreate)
	mux.Get("/orders/:id", api.trace(api.OrderView))
	mux.Get("/orders/:order_id/payments", api.PaymentList)
	mux.Post("/orders/:order_id/payments", api.PaymentCreate)
	mux.Get("/vatnumbers/:number", api.VatnumberLookup)

	corsHandler := cors.New(cors.Options{
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	})

	api.handler = corsHandler.Handler(mux)
	return api
}

func (a *API) trace(f func(context.Context, http.ResponseWriter, *http.Request)) func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		id := uuid.NewV4().String()
		log := a.log.WithField("request_id", id)

		ctx = WithRequestID(ctx, id)
		ctx = WithLogger(ctx, log)
		ctx = WithConfig(ctx, a.config)

		log = log.WithFields(logrus.Fields{
			"method": r.Method,
			"url":    r.URL.String(),
		})

		// optionally add the user id stuff
		claims := Claims(ctx)
		if token != nil {
			log = log.WithFields(logrus.Fields{
				"user_id":     claims.ID,
				"user_groups": claims.Groups,
			})
		}

		log.Debug("request started")
		defer log.Debug("request completed")
		f(ctx, w, r)
	}
}
