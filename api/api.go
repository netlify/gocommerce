package api

import (
	"context"
	"net/http"
	"regexp"

	"github.com/pkg/errors"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	"github.com/pborman/uuid"
	"github.com/rs/cors"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
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

	r := chi.NewRouter()
	r.Use(requestIDCtx)
	r.Use(newStructuredLogger(logrus.StandardLogger()))
	r.Use(middleware.Recoverer)
	r.Use(api.instanceConfigCtx)
	r.Use(withTokenCtx)

	// endpoints
	r.Get("/", api.Index)

	r.Route("/orders", func(r chi.Router) {
		r.With(authRequired).Get("/", api.OrderList)
		r.Post("/", api.OrderCreate)

		r.Route("/{order_id}", func(r chi.Router) {
			r.Use(api.orderCtx)
			// TODO should anonymous order viewing be allowed?
			r.With(authRequired).Get("/", api.OrderView)
			r.With(adminRequired).Put("/", api.OrderUpdate)

			r.Route("/payments", func(r chi.Router) {
				r.With(authRequired).Get("/", api.PaymentListForOrder)
				r.Post("/", api.PaymentCreate)
			})

			r.Get("/downloads", api.DownloadList)
			r.Post("/receipt", api.ResendOrderReceipt)
		})
	})

	r.Route("/users", func(r chi.Router) {
		r.Use(authRequired)
		r.With(adminRequired).Get("/", api.UserList)

		r.Route("/{user_id}", func(r chi.Router) {
			r.Use(api.userCtx)
			r.Use(ensureUserAccess)

			r.Get("/", api.UserView)
			r.With(adminRequired).Delete("/", api.UserDelete)

			r.Get("/payments", api.PaymentListForUser)
			r.Get("/orders", api.OrderList)

			r.Route("/addresses", func(r chi.Router) {
				r.Get("/", api.AddressList)
				r.With(adminRequired).Post("/", api.CreateNewAddress)
				r.Route("/{addr_id}", func(r chi.Router) {
					r.Get("/", api.AddressView)
					r.With(adminRequired).Delete("/", api.AddressDelete)
				})
			})
		})
	})

	r.Route("/downloads", func(r chi.Router) {
		r.With(authRequired).Get("/", api.DownloadList)
		r.Get("/{download_id}", api.DownloadURL)
	})

	r.Route("/vatnumbers", func(r chi.Router) {
		r.Get("/{vat_number}", api.VatNumberLookup)
	})

	r.Route("/payments", func(r chi.Router) {
		r.Use(adminRequired)

		r.Get("/", api.PaymentList)
		r.Route("/{payment_id}", func(r chi.Router) {
			r.Get("/", api.PaymentView)
			r.Post("/refund", api.PaymentRefund)
		})
	})

	r.Route("/paypal", func(r chi.Router) {
		r.Post("/", api.PreauthorizePayment)
		// TODO is this needed? I did not see a use case in the PayPal payment flow.
		// r.Get("/{payment_id}", api.PayPalGetPayment)
	})

	r.Route("/reports", func(r chi.Router) {
		r.Use(adminRequired)

		r.Get("/sales", api.SalesReport)
		r.Get("/products", api.ProductsReport)
	})

	r.Route("/coupons", func(r chi.Router) {
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

func requestIDCtx(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewRandom().String()
		ctx := r.Context()
		ctx = gcontext.WithRequestID(ctx, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

func (a *API) instanceConfigCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if gcontext.GetPaymentProviders(ctx) == nil {
			internalServerError(w, "No payment providers configured")
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
