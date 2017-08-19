package api

import (
	"context"
	"net/http"
	"regexp"

	"github.com/pkg/errors"

	"github.com/jinzhu/gorm"
	"github.com/pborman/uuid"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"

	"github.com/go-chi/chi"
	"github.com/netlify/gocommerce/assetstores"
	"github.com/netlify/gocommerce/conf"
	gcontext "github.com/netlify/gocommerce/context"
	"github.com/netlify/gocommerce/mailer"
	"github.com/netlify/netlify-commons/graceful"
)

const (
	defaultVersion = "unknown version"
)

var (
	bearerRegexp = regexp.MustCompile(`^(?:B|b)earer (\S+$)`)
)

// API is the main REST API
type API struct {
	handler    http.Handler
	db         *gorm.DB
	config     *conf.GlobalConfiguration
	httpClient *http.Client
	version    string
}

// ListenAndServe starts the REST API.
func (a *API) ListenAndServe(hostAndPort string) {
	log := logrus.WithField("component", "api")
	server := graceful.NewGracefulServer(a.handler, log)
	if err := server.Bind(hostAndPort); err != nil {
		log.WithError(err).Fatal("http server bind failed")
	}

	if err := server.Listen(); err != nil {
		log.WithError(err).Fatal("http server listen failed")
	}
}

// NewAPI instantiates a new REST API using the default version.
func NewAPI(globalConfig *conf.GlobalConfiguration, config *conf.Configuration, db *gorm.DB) *API {
	return NewSingleTenantAPIWithVersion(globalConfig, config, db, defaultVersion)
}

// NewSingleTenantAPIWithVersion creates a single-tenant version of the REST API
func NewSingleTenantAPIWithVersion(globalConfig *conf.GlobalConfiguration, config *conf.Configuration, db *gorm.DB, version string) *API {
	ctx, err := withTenantConfig(context.Background(), config)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to get tenant configuration")
	}

	return NewAPIWithVersion(ctx, globalConfig, db, version)
}

// NewAPIWithVersion instantiates a new REST API.
func NewAPIWithVersion(ctx context.Context, config *conf.GlobalConfiguration, db *gorm.DB, version string) *API {
	api := &API{
		config:     config,
		db:         db,
		httpClient: &http.Client{},
		version:    version,
	}

	r := newRouter()
	r.Use(withRequestID)
	r.UseBypass(newStructuredLogger(logrus.StandardLogger()))
	r.Use(recoverer)
	r.Use(api.instanceConfigCtx)
	r.Use(withToken)

	// endpoints
	r.Get("/", api.Index)

	r.Route("/orders", api.orderRoutes)
	r.Route("/users", api.userRoutes)

	r.Route("/downloads", func(r *router) {
		r.With(authRequired).Get("/", api.DownloadList)
		r.Get("/{download_id}", api.DownloadURL)
	})

	r.Route("/vatnumbers", func(r *router) {
		r.Get("/{vat_number}", api.VatNumberLookup)
	})

	r.Route("/payments", func(r *router) {
		r.Use(adminRequired)

		r.Get("/", api.PaymentList)
		r.Route("/{payment_id}", func(r *router) {
			r.Get("/", api.PaymentView)
			r.With(addGetBody).Post("/refund", api.PaymentRefund)
		})
	})

	r.Route("/paypal", func(r *router) {
		r.With(addGetBody).Post("/", api.PreauthorizePayment)
	})

	r.Route("/reports", func(r *router) {
		r.Use(adminRequired)

		r.Get("/sales", api.SalesReport)
		r.Get("/products", api.ProductsReport)
	})

	r.Route("/coupons", func(r *router) {
		r.Get("/{coupon_code}", api.CouponView)
	})

	r.With(authRequired).Post("/claim", api.ClaimOrders)

	corsHandler := cors.New(cors.Options{
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link", "X-Total-Count"},
		AllowCredentials: true,
	})

	api.handler = corsHandler.Handler(chi.ServerBaseContext(r, ctx))

	return api
}

func (a *API) orderRoutes(r *router) {
	r.With(authRequired).Get("/", a.OrderList)
	r.Post("/", a.OrderCreate)

	r.Route("/{order_id}", func(r *router) {
		r.Use(a.withOrderID)
		r.Get("/", a.OrderView)
		r.With(adminRequired).Put("/", a.OrderUpdate)

		r.Route("/payments", func(r *router) {
			r.With(authRequired).Get("/", a.PaymentListForOrder)
			r.With(addGetBody).Post("/", a.PaymentCreate)
		})

		r.Get("/downloads", a.DownloadList)
		r.Route("/receipt", func(r *router) {
			r.Get("/", a.ReceiptView)
			r.Post("/", a.ResendOrderReceipt)
			r.Get("/*", a.ReceiptView)
		})

	})
}

func (a *API) userRoutes(r *router) {
	r.Use(authRequired)
	r.With(adminRequired).Get("/", a.UserList)

	r.Route("/{user_id}", func(r *router) {
		r.Use(a.withUserID)
		r.Use(ensureUserAccess)

		r.Get("/", a.UserView)
		r.With(adminRequired).Delete("/", a.UserDelete)

		r.Get("/payments", a.PaymentListForUser)
		r.Get("/orders", a.OrderList)

		r.Route("/addresses", func(r *router) {
			r.Get("/", a.AddressList)
			r.With(adminRequired).Post("/", a.CreateNewAddress)
			r.Route("/{addr_id}", func(r *router) {
				r.Get("/", a.AddressView)
				r.With(adminRequired).Delete("/", a.AddressDelete)
			})
		})
	})
}

func withRequestID(w http.ResponseWriter, r *http.Request) (context.Context, error) {
	id := uuid.NewRandom().String()
	ctx := r.Context()
	ctx = gcontext.WithRequestID(ctx, id)
	return ctx, nil
}

func (a *API) instanceConfigCtx(w http.ResponseWriter, r *http.Request) (context.Context, error) {
	ctx := r.Context()
	if gcontext.GetPaymentProviders(ctx) == nil {
		return nil, internalServerError("No payment providers configured")
	}
	return ctx, nil
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

	provs, err := createPaymentProviders(config)
	if err != nil {
		return nil, errors.Wrap(err, "error creating payment providers")
	}
	if len(provs) == 0 {
		return nil, errors.Wrap(err, "No payment providers enabled")
	}
	ctx = gcontext.WithPaymentProviders(ctx, provs)

	return ctx, nil
}
