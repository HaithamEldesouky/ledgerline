package httpapi

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// statusRecorder wraps http.ResponseWriter to capture the status code written,
// so middleware can log and measure responses.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}

// requestLogger logs one structured line per request.
func requestLogger(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.LogAttrs(r.Context(), slog.LevelInfo, "http_request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
			slog.String("remote", clientIP(r)),
		)
	})
}

// recoverMiddleware converts a panic into a 500 response instead of crashing
// the process.
func recoverMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.ErrorContext(r.Context(), "panic recovered", slog.Any("panic", rec))
				writeJSON(w, http.StatusInternalServerError, errorBody{
					Error: errorDetail{
						Code:    "INTERNAL_SERVER_ERROR",
						Message: "an unexpected error occurred",
					},
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RateLimiter applies a token-bucket rate limit per client IP.
type RateLimiter struct {
	rps   rate.Limit
	burst int

	mu      sync.Mutex
	clients map[string]*rateClient
	stop    chan struct{}
}

type rateClient struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a per-IP limiter and starts a background reaper that
// evicts idle clients.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		rps:     rate.Limit(rps),
		burst:   burst,
		clients: make(map[string]*rateClient),
		stop:    make(chan struct{}),
	}
	go rl.reap()
	return rl
}

func (rl *RateLimiter) reap() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case <-ticker.C:
			rl.mu.Lock()
			for ip, c := range rl.clients {
				if time.Since(c.lastSeen) > 3*time.Minute {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// Close stops the background reaper.
func (rl *RateLimiter) Close() { close(rl.stop) }

func (rl *RateLimiter) limiterFor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	c, ok := rl.clients[ip]
	if !ok {
		c = &rateClient{limiter: rate.NewLimiter(rl.rps, rl.burst)}
		rl.clients[ip] = c
	}
	c.lastSeen = time.Now()
	return c.limiter
}

// Middleware enforces the rate limit, exempting health and metrics endpoints
// so probes and scrapes are never throttled.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/health") || path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		if !rl.limiterFor(clientIP(r)).Allow() {
			writeJSON(w, http.StatusTooManyRequests, errorBody{
				Error: errorDetail{
					Code:    "RATE_LIMITED",
					Message: "too many requests",
				},
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the best-effort client IP, honouring X-Forwarded-For when
// present (the service is expected to run behind a trusted proxy/ingress).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
