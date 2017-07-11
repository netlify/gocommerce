package api

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/pkg/errors"

	"github.com/Sirupsen/logrus"
	"github.com/guregu/kami"
	"github.com/jinzhu/gorm"
	"github.com/pborman/uuid"
	"github.com/rs/cors"
	"github.com/zenazn/goji/web/mutil"

	"github.com/netlify/gocommerce/assetstores"
	"github.com/netlify/gocommerce/conf"
	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/mailer"
)

var (
	defaultVersion = "unknown version"
	bearerRegexp   = regexp.MustCompile(`^(?:B|b)earer (\S+$)`)
)

// API is the main REST API
type API struct {
	handler    http.Handler
	db         *gorm.DB
	config     *conf.GlobalConfiguration
	httpClient *http.Client
	log        *logrus.Entry
	version    string
}

// ListenAndServe starts the REST API.
func (a *API) ListenAndServe(hostAndPort string) error {
	return http.ListenAndServe(hostAndPort, a.handler)
}

// NewAPI instantiates a new REST API using the default version.
func NewAPI(globalConfig *conf.GlobalConfiguration, config *conf.Configuration, db *gorm.DB) *API {
	return NewSingleTenantAPIWithVersion(globalConfig, config, db, defaultVersion)
}

// NewMultiTenantAPI creates a new REST API to serve multiple tenants.
func NewMultiTenantAPI(globalConfig *conf.GlobalConfiguration, db *gorm.DB) *API {
	return NewAPIWithVersion(context.Background(), globalConfig, db, defaultVersion)
}

// NewSingleTenantAPIWithVersion creates a single-tenant version of the REST API
func NewSingleTenantAPIWithVersion(globalConfig *conf.GlobalConfiguration, config *conf.Configuration, db *gorm.DB, version string) *API {
	ctx := context.Background()
	ctx, err := withTenantConfig(ctx, config)
	if err != nil {
		logrus.Fatalf("%+v", err)
	}

	return NewAPIWithVersion(ctx, globalConfig, db, version)
}

// NewAPIWithVersion instantiates a new REST API.
func NewAPIWithVersion(ctx context.Context, config *conf.GlobalConfiguration, db *gorm.DB, version string) *API {
	api := &API{
		log:        logrus.WithField("component", "api"),
		config:     config,
		db:         db,
		httpClient: &http.Client{},
		version:    version,
	}

	mux := kami.New()
	mux.Context = ctx
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

	mux.Post("/paypal", api.PreauthorizePayment)
	// TODO is this needed? I did not see a use case in the PayPal payment flow.
	// mux.Get("/paypal/:payment_id", api.PayPalGetPayment)

	mux.Get("/reports/sales", api.SalesReport)
	mux.Get("/reports/products", api.ProductsReport)

	mux.Get("/coupons/:code", api.CouponView)

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
	log := gcontext.GetLogger(ctx).WithField("status", wp.Status())

	start := gcontext.GetStartTime(ctx)
	if start != nil {
		log = log.WithField("duration", time.Since(*start))
	}

	log.Infof("Completed request %s. path: %s, method: %s, status: %d", gcontext.GetRequestID(ctx), r.URL.Path, r.Method, wp.Status())
}

func (a *API) populateContext(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	id := uuid.NewRandom().String()
	log := a.log.WithField("request_id", id)

	log = log.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	})

	ctx = gcontext.WithRequestID(ctx, id)
	ctx = gcontext.WithLogger(ctx, log)
	ctx = gcontext.WithStartTime(ctx, time.Now())

	instanceID := r.Header.Get("x-nf-id")
	if instanceID != "" {
		// TODO populate config
		// env := r.Header.Get("x-nf-env")
		// var config *conf.Configuration
		// var err error
		// ctx, err = withTenantConfig(ctx, config)
		// if err != nil {
		// 	internalServerError(w, err.Error())
		// 	return nil
		// }
	}

	// safety check in case of multi-tenant but missing X-NF-* headers
	if gcontext.GetPaymentProvider(ctx) == nil {
		internalServerError(w, "No payment provider configured")
		return nil
	}

	log.Info("request started")
	return ctx
}

func withTenantConfig(ctx context.Context, config *conf.Configuration) (context.Context, error) {
	ctx = gcontext.WithConfig(ctx, config)
	ctx = gcontext.WithCoupons(ctx, config)

	mailer := mailer.NewMailer(config)
	ctx = gcontext.WithMailer(ctx, mailer)

	store, err := assetstores.NewStore(config)
	if err != nil {
		return nil, errors.Wrap(err, "Error initializing asset store")
	}
	ctx = gcontext.WithAssetStore(ctx, store)

	prov, err := createPaymentProvider(config)
	if err != nil {
		return nil, errors.Wrap(err, "error creating payment provider")
	}
	ctx = gcontext.WithPaymentProvider(ctx, prov)

	return ctx, nil
}
