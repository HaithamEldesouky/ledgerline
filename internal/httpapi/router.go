package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/haithamEldesouky/ledgerline/internal/domain"
)

// RouterDeps are the dependencies required to build the HTTP handler.
type RouterDeps struct {
	Service     *domain.Service
	Repository  domain.Repository
	Logger      *slog.Logger
	Metrics     *Metrics
	RateLimiter *RateLimiter
}

// NewRouter wires the routes and middleware into a single http.Handler.
//
// Middleware order (outermost first): request logging, panic recovery, rate
// limiting, then routing. Each business route is individually instrumented for
// Prometheus metrics using its static path pattern.
func NewRouter(deps RouterDeps) http.Handler {
	api := &API{svc: deps.Service, repo: deps.Repository, log: deps.Logger}

	mux := http.NewServeMux()

	type route struct {
		method  string
		pattern string
		handler http.HandlerFunc
	}
	routes := []route{
		{http.MethodPost, "/api/v1/accounts", api.createAccount},
		{http.MethodGet, "/api/v1/accounts", api.listAccounts},
		{http.MethodGet, "/api/v1/accounts/{id}", api.getAccount},
		{http.MethodGet, "/api/v1/accounts/{id}/transactions", api.listAccountEntries},
		{http.MethodPost, "/api/v1/transfers", api.createTransfer},
		{http.MethodGet, "/health/live", api.live},
		{http.MethodGet, "/health/ready", api.ready},
		{http.MethodGet, "/openapi.yaml", api.openapiSpec},
		{http.MethodGet, "/docs", api.docs},
	}
	for _, rt := range routes {
		mux.Handle(
			rt.method+" "+rt.pattern,
			deps.Metrics.Instrument(rt.method, rt.pattern, rt.handler),
		)
	}

	// The metrics endpoint is intentionally not self-instrumented.
	mux.Handle(http.MethodGet+" /metrics", deps.Metrics.Handler())

	var handler http.Handler = mux
	handler = deps.RateLimiter.Middleware(handler)
	handler = recoverMiddleware(deps.Logger, handler)
	handler = requestLogger(deps.Logger, handler)
	return handler
}
