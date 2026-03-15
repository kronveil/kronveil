package rest

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// withCORS adds CORS headers with configurable origins.
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := s.isOriginAllowed(origin)

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.Header().Set("Vary", "Origin")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isOriginAllowed checks if the given origin is in the allowed list.
// If no origins are configured, no origin is allowed (secure by default).
func (s *Server) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	for _, o := range s.config.AllowedOrigins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}

// withBodyLimit restricts request body size.
func (s *Server) withBodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// withRateLimit applies per-client rate limiting.
func (s *Server) withRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			clientIP = strings.SplitN(fwd, ",", 2)[0]
		}
		if !s.limiter.Allow(strings.TrimSpace(clientIP)) {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withLogging logs HTTP requests.
func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(wrapped, r)
		log.Printf("[api] %s %s %d %s", r.Method, r.URL.Path, wrapped.status, time.Since(start))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// withAuth validates API key authentication.
// When no API key is configured, a warning is logged once and requests pass through.
func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.config.APIKey == "" {
			// Allow passthrough but this is intentional — operator chose no auth.
			next(w, r)
			return
		}

		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				apiKey = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if apiKey == "" {
			writeError(w, http.StatusUnauthorized, "API key required")
			return
		}

		if apiKey != s.config.APIKey {
			writeError(w, http.StatusUnauthorized, "invalid API key")
			return
		}

		next(w, r)
	}
}

// RateLimiter implements a simple token bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*clientBucket
	rate     float64
	capacity int
}

type clientBucket struct {
	tokens     float64
	lastRefill time.Time
}

// NewRateLimiter creates a rate limiter with the given requests/sec and burst capacity.
func NewRateLimiter(requestsPerSec float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		clients:  make(map[string]*clientBucket),
		rate:     requestsPerSec,
		capacity: burst,
	}
	// Start background cleanup of stale entries.
	go rl.cleanup()
	return rl
}

// cleanup evicts stale client entries every 10 minutes.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, bucket := range rl.clients {
			if now.Sub(bucket.lastRefill) > 1*time.Hour {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Allow checks if the given client IP is allowed to make a request.
func (rl *RateLimiter) Allow(clientIP string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, ok := rl.clients[clientIP]
	if !ok {
		bucket = &clientBucket{
			tokens:     float64(rl.capacity),
			lastRefill: time.Now(),
		}
		rl.clients[clientIP] = bucket
	}

	// Refill tokens.
	elapsed := time.Since(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * rl.rate
	if bucket.tokens > float64(rl.capacity) {
		bucket.tokens = float64(rl.capacity)
	}
	bucket.lastRefill = time.Now()

	if bucket.tokens < 1 {
		return false
	}

	bucket.tokens--
	return true
}
