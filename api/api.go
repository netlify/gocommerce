package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/Sirupsen/logrus"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/guregu/kami"
	"github.com/jinzhu/gorm"
	"github.com/netlify/netlify-commerce/conf"
	"github.com/netlify/netlify-commerce/mailer"
	"github.com/pborman/uuid"
	"github.com/rs/cors"
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

type ResponseProxy struct {
	http.ResponseWriter
	statusCode int
}

func (rp ResponseProxy) WriteHeader(status int) {
	rp.setStatusCode(status)
	rp.ResponseWriter.WriteHeader(status)
}

func (rp *ResponseProxy) setStatusCode(code int) {
	rp.statusCode = code
}

type JWTClaims struct {
	ID     string   `json:"id"`
	Email  string   `json:"email"`
	Groups []string `json:"groups"`
	*jwt.StandardClaims
}

func (a *API) withToken(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ctx
	}

	matches := bearerRegexp.FindStringSubmatch(authHeader)
	if len(matches) != 2 {
		UnauthorizedError(w, "Bad authentication header")
		return nil
	}

	token, err := jwt.ParseWithClaims(matches[1], &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Header["alg"] != "HS256" {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(getConfig(ctx).JWT.Secret), nil
	})
	if err != nil {
		UnauthorizedError(w, fmt.Sprintf("Invalid token: %v", err))
		return nil
	}
	claims := token.Claims.(*JWTClaims)
	if claims.StandardClaims.ExpiresAt < time.Now().Unix() {
		UnauthorizedError(w, fmt.Sprintf("Expired token: %v", err))
		return nil
	}

	return context.WithValue(ctx, "jwt", token)
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
	api := &API{config: config, db: db, mailer: mailer, httpClient: &http.Client{}}

	mux := kami.New()
	mux.Use("/", api.withToken)

	// endpoints
	mux.Get("/", api.trace(api.Index))
	mux.Get("/orders", api.trace(api.OrderList))
	mux.Post("/orders", api.trace(api.OrderCreate))
	mux.Get("/orders/:id", api.trace(api.OrderView))
	mux.Get("/orders/:order_id/payments", api.trace(api.PaymentList))
	mux.Post("/orders/:order_id/payments", api.trace(api.PaymentCreate))
	mux.Get("/vatnumbers/:number", api.trace(api.VatnumberLookup))

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
		id := uuid.NewRandom().String()
		log := a.log.WithField("request_id", id)

		log = log.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
		})

		// optionally add the user id stuff
		claims := getClaims(ctx)
		if claims != nil {
			log = log.WithFields(logrus.Fields{
				"claim_id":     claims.ID,
				"claim_groups": claims.Groups,
				"claim_email":  claims.Email,
			})
		}

		ctx = withRequestID(ctx, id)
		ctx = withLogger(ctx, log)
		ctx = withConfig(ctx, a.config)

		rp := ResponseProxy{ResponseWriter: w}
		startTime := time.Now()

		log.Info("request started")
		defer log.WithFields(logrus.Fields{
			"status":   rp.statusCode,
			"duration": time.Since(startTime),
		}).Infof("request completed: %d", rp.statusCode)

		f(ctx, rp, r)
	}
}
