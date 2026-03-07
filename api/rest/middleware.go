package rest

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// withCORS adds CORS headers.
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
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
func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.config.APIKey == "" {
			next(w, r)
			return
		}

		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}

		if apiKey != s.config.APIKey {
			writeError(w, http.StatusUnauthorized, "invalid or missing API key")
			return
		}

		next(w, r)
	}
}

// rateLimiter implements a simple token bucket rate limiter.
type rateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*clientBucket
	rate     float64
	capacity int
}

type clientBucket struct {
	tokens     float64
	lastRefill time.Time
}

func newRateLimiter(requestsPerSec float64, burst int) *rateLimiter {
	return &rateLimiter{
		clients:  make(map[string]*clientBucket),
		rate:     requestsPerSec,
		capacity: burst,
	}
}

func (rl *rateLimiter) allow(clientIP string) bool {
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
