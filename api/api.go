package api

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/Sirupsen/logrus"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/guregu/kami"
	"github.com/jinzhu/gorm"
	"github.com/pborman/uuid"
	"github.com/rs/cors"
	"github.com/zenazn/goji/web/mutil"

	"github.com/netlify/netlify-commerce/assetstores"
	"github.com/netlify/netlify-commerce/conf"
	"github.com/netlify/netlify-commerce/mailer"

	paypalsdk "github.com/logpacker/PayPal-Go-SDK"
)

var (
	defaultVersion = "unknown version"
	bearerRegexp   = regexp.MustCompile(`^(?:B|b)earer (\S+$)`)
)

// API is the main REST API
type API struct {
	handler    http.Handler
	db         *gorm.DB
	paypal     *paypalsdk.Client
	config     *conf.Configuration
	mailer     *mailer.Mailer
	httpClient *http.Client
	log        *logrus.Entry
	assets     assetstores.Store
	version    string
}

type JWTClaims struct {
	ID           string                 `json:"id"`
	Email        string                 `json:"email"`
	AppMetaData  map[string]interface{} `json:"app_metadata"`
	UserMetaData map[string]interface{} `json:"user_metadata"`
	*jwt.StandardClaims
}

func (a *API) withToken(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	log := getLogger(ctx)
	config := getConfig(ctx)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Info("Making unauthenticated request")
		return ctx
	}

	matches := bearerRegexp.FindStringSubmatch(authHeader)
	if len(matches) != 2 {
		log.Info("Invalid auth header format: " + authHeader)
		unauthorizedError(w, "Bad authentication header")
		return nil
	}

	token, err := jwt.ParseWithClaims(matches[1], &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Header["alg"] != jwt.SigningMethodHS256.Name {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.JWT.Secret), nil
	})
	if err != nil {
		log.Infof("Invalid token: %v", err)
		unauthorizedError(w, "Invalid token")
		return nil
	}

	claims := token.Claims.(*JWTClaims)
	if claims.StandardClaims.ExpiresAt < time.Now().Unix() {
		msg := fmt.Sprintf("Token expired at %v", time.Unix(claims.StandardClaims.ExpiresAt, 0))
		log.Info(msg)
		unauthorizedError(w, msg)
		return nil
	}

	isAdmin := false
	roles, ok := claims.AppMetaData["roles"]
	if ok {
		roleStrings, _ := roles.([]interface{})
		for _, data := range roleStrings {
			role, _ := data.(string)
			if role == config.JWT.AdminGroupName {
				isAdmin = true
				break
			}
		}
	}

	log = log.WithFields(logrus.Fields{
		"claims_id":    claims.ID,
		"claims_email": claims.Email,
		"roles":        roles,
		"is_admin":     isAdmin,
	})

	log.Info("successfully parsed claims")
	ctx = withAdminFlag(ctx, isAdmin)
	ctx = withLogger(ctx, log)

	return withToken(ctx, token)
}

// ListenAndServe starts the REST API
func (a *API) ListenAndServe(hostAndPort string) error {
	return http.ListenAndServe(hostAndPort, a.handler)
}

func NewAPI(config *conf.Configuration, db *gorm.DB, paypal *paypalsdk.Client, mailer *mailer.Mailer, store assetstores.Store) *API {
	return NewAPIWithVersion(config, db, paypal, mailer, store, defaultVersion)
}

// NewAPIWithVersion instantiates a new REST API
func NewAPIWithVersion(config *conf.Configuration, db *gorm.DB, paypal *paypalsdk.Client, mailer *mailer.Mailer, assets assetstores.Store, version string) *API {
	api := &API{
		log:        logrus.WithField("component", "api"),
		config:     config,
		db:         db,
		paypal:     paypal,
		mailer:     mailer,
		httpClient: &http.Client{},
		assets:     assets,
		version:    version}

	mux := kami.New()
	mux.Use("/", api.populateContext)
	mux.Use("/", api.withToken)
	mux.LogHandler = api.logCompleted

	// endpoints
	mux.Get("/", api.Index)

	mux.Get("/orders", api.OrderList)
	mux.Post("/orders", api.OrderCreate)
	mux.Get("/orders/:id", api.OrderView)
	mux.Put("/orders/:id", api.OrderUpdate)
	mux.Get("/orders/:order_id/payments", api.PaymentListForOrder)
	mux.Post("/orders/:order_id/payments", api.PaymentCreate)
	mux.Post("/orders/:order_id/receipt", api.ResendOrderReceipt)

	mux.Get("/users", api.UserList)
	mux.Get("/users/:user_id", api.UserView)
	mux.Get("/users/:user_id/payments", api.PaymentListForUser)
	mux.Delete("/users/:user_id", api.UserDelete)
	mux.Get("/users/:user_id/addresses", api.AddressList)
	mux.Get("/users/:user_id/addresses/:addr_id", api.AddressView)
	mux.Delete("/users/:user_id/addresses/:addr_id", api.AddressDelete)
	mux.Get("/users/:user_id/orders", api.OrderList)

	mux.Get("/downloads/:id", api.DownloadURL)
	mux.Get("/downloads", api.DownloadList)
	mux.Get("/orders/:order_id/downloads", api.DownloadList)

	mux.Get("/vatnumbers/:number", api.VatnumberLookup)

	mux.Get("/payments", api.PaymentList)
	mux.Get("/payments/:pay_id", api.PaymentView)
	mux.Post("/payments/:pay_id/refund", api.PaymentRefund)

	mux.Post("/paypal", api.PaypalCreatePayment)
	mux.Get("/paypal/:payment_id", api.PaypalGetPayment)

	mux.Get("/reports/sales", api.SalesReport)
	mux.Get("/reports/products", api.ProductsReport)

	mux.Post("/claim", api.ClaimOrders)

	corsHandler := cors.New(cors.Options{
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link", "X-Total-Count"},
		AllowCredentials: true,
	})

	api.handler = corsHandler.Handler(mux)

	return api
}

func (a *API) logCompleted(ctx context.Context, wp mutil.WriterProxy, r *http.Request) {
	log := getLogger(ctx).WithField("status", wp.Status())

	start := getStartTime(ctx)
	if start != nil {
		log = log.WithField("duration", time.Since(*start))
	}

	log.Infof("Completed request %s. path: %s, method: %s, status: %d", getRequestID(ctx), r.URL.Path, r.Method, wp.Status())
}

func (a *API) populateContext(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	id := uuid.NewRandom().String()
	log := a.log.WithField("request_id", id)

	log = log.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	})

	ctx = withRequestID(ctx, id)
	ctx = withLogger(ctx, log)
	ctx = withConfig(ctx, a.config)
	ctx = withStartTime(ctx, time.Now())
	ctx = withPayer(ctx, PaypalChargerType, &paypalProvider{a.paypal})
	ctx = withPayer(ctx, StripeChargerType, &stripeProvider{})

	log.Info("request started")
	return ctx
}
