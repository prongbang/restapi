package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"restapi/conf"

	"github.com/google/uuid"
	"github.com/guregu/kami"
	"github.com/rs/cors"
	"github.com/zenazn/goji/web/mutil"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type API struct {
	log     *logrus.Entry
	config  *conf.Config
	port    int
	handler http.Handler
	db      *gorm.DB
	version string
}

type JWTClaims struct {
	jwt.StandardClaims
	Email  string   `json:"email"`
	Groups []string `json:"groups"`
}

var bearerRegexp = regexp.MustCompile(`^(?:B|b)earer (\S+$)`)
var signingMethod = jwt.SigningMethodHS256

func NewAPI(config *conf.Config, version string) *API {
	api := &API{
		log:     logrus.WithField("component", "api"),
		config:  config,
		port:    config.Port,
		version: version,
	}

	k := kami.New()
	k.LogHandler = logCompleted

	k.Get("/", api.hello)

	// k.Use("/subscriptions/", api.populateConfig)
	// k.Use("/subscriptions", api.populateConfig)

	k.Get("/subscriptions", listSubs)
	k.Get("/subscriptions/:type", viewSub)
	k.Put("/subscriptions/:type", createOrModSub)
	k.Delete("/subscriptions/:type", deleteSub)

	corsHandler := cors.New(cors.Options{
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	})

	api.handler = corsHandler.Handler(k)
	return api
}

func (a *API) Serve() error {
	l := fmt.Sprintf(":%d", a.port)
	a.log.Infof("GoJoin API started on: %s", l)
	return http.ListenAndServe(l, a.handler)
}

func logCompleted(ctx context.Context, wp mutil.WriterProxy, r *http.Request) {
	log := getLogger(ctx).WithField("status", wp.Status())

	start := getStartTime(ctx)
	if start != nil {
		log = log.WithField("duration", time.Since(*start).Nanoseconds())
	}

	log.Infof("Completed request %s. path: %s, method: %s, status: %d", getRequestID(ctx), r.URL.Path, r.Method, wp.Status())
}

func (a *API) populateConfig(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	reqID, _ := uuid.NewRandom()
	log := a.log.WithFields(logrus.Fields{
		"request_id": string(reqID[:]),
		"method":     r.Method,
		"path":       r.URL.Path,
	})
	log.Info("Started request")

	ctx = setRequestID(ctx, string(reqID[:]))
	ctx = setStartTime(ctx, time.Now())
	ctx = setConfig(ctx, a.config)
	ctx = setDB(ctx, a.db)

	token, err := extractToken(a.config.JWTSecret, r)
	if err != nil {
		log.WithError(err).Info("Failed to parse token")
		sendJSON(w, err.Code, err)
		return nil
	}

	if token == nil {
		log.Info("Attempted to make unauthenticated request")
		writeError(w, http.StatusBadRequest, "Must provide a valid JWT Token")
		return nil
	}

	claims := token.Claims.(*JWTClaims)
	if claims.Subject == "" {
		log.Info("JWT token did not contain a sub")
		writeError(w, http.StatusBadRequest, "JWT Token must contain a sub")
		return nil
	}

	adminFlag := false
	for _, g := range claims.Groups {
		if g == a.config.AdminGroupName {
			adminFlag = true
			break
		}
	}
	log = log.WithFields(logrus.Fields{
		"is_admin": adminFlag,
		"user_id":  claims.Subject,
	})
	ctx = setAdminFlag(ctx, adminFlag)
	ctx = setToken(ctx, token)
	ctx = setLogger(ctx, log)

	return ctx
}

func extractToken(secret string, r *http.Request) (*jwt.Token, *HTTPError) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, nil
	}

	matches := bearerRegexp.FindStringSubmatch(authHeader)
	if len(matches) != 2 {
		return nil, httpError(http.StatusBadRequest, "Bad authentication header")
	}

	token, err := jwt.ParseWithClaims(matches[1], &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Header["alg"] != signingMethod.Name {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, httpError(http.StatusUnauthorized, "Invalid Token")
	}

	claims := token.Claims.(*JWTClaims)
	if claims.StandardClaims.ExpiresAt < time.Now().Unix() {
		return nil, httpError(http.StatusUnauthorized, fmt.Sprintf("Token expired at %v", time.Unix(claims.StandardClaims.ExpiresAt, 0)))
	}
	return token, nil
}

func (a *API) hello(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	sendJSON(w, http.StatusOK, map[string]string{
		"version":     a.version,
		"application": "gojoin",
	})
}

func sendJSON(w http.ResponseWriter, status int, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.Encode(obj)
}
