package rest

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testServer(apiKey string) *Server {
	return &Server{
		config: Config{APIKey: apiKey},
	}
}

func testServerWithOrigins(origins []string) *Server {
	return &Server{
		config: Config{AllowedOrigins: origins},
	}
}

func TestWithCORS_AllowedOrigin(t *testing.T) {
	s := testServerWithOrigins([]string{"https://app.kronveil.io"})
	handler := s.withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://app.kronveil.io")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://app.kronveil.io" {
		t.Error("expected matching CORS Allow-Origin header")
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected CORS Allow-Methods header")
	}
}

func TestWithCORS_DisallowedOrigin(t *testing.T) {
	s := testServerWithOrigins([]string{"https://app.kronveil.io"})
	handler := s.withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("disallowed origin should not get CORS header")
	}
}

func TestWithCORS_WildcardOrigin(t *testing.T) {
	s := testServerWithOrigins([]string{"*"})
	handler := s.withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://anything.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://anything.com" {
		t.Error("wildcard should allow any origin")
	}
}

func TestWithCORS_NoOriginsConfigured(t *testing.T) {
	s := testServer("")
	handler := s.withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://test.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("no origins configured should block all")
	}
}

func TestWithCORS_OptionsReturns204(t *testing.T) {
	s := testServerWithOrigins([]string{"*"})
	handler := s.withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://test.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS should return 204, got %d", w.Code)
	}
}

func TestWithAuth_NoKey_PassesThrough(t *testing.T) {
	s := testServer("") // No API key configured.
	handler := s.withAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when no API key configured, got %d", w.Code)
	}
}

func TestWithAuth_ValidKey(t *testing.T) {
	s := testServer("secret-key")
	handler := s.withAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "secret-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid key, got %d", w.Code)
	}
}

func TestWithAuth_ValidBearerKey(t *testing.T) {
	s := testServer("secret-key")
	handler := s.withAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer secret-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid Bearer key, got %d", w.Code)
	}
}

func TestWithAuth_InvalidKey(t *testing.T) {
	s := testServer("secret-key")
	handler := s.withAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid key, got %d", w.Code)
	}
}

func TestWithAuth_MissingKey(t *testing.T) {
	s := testServer("secret-key")
	handler := s.withAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing key, got %d", w.Code)
	}
}

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	rl := NewRateLimiter(100, 5) // 100 req/s, burst of 5

	for i := 0; i < 5; i++ {
		if !rl.Allow("client1") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := NewRateLimiter(1, 2) // 1 req/s, burst of 2

	// Use burst.
	rl.Allow("client1")
	rl.Allow("client1")

	// Next should be blocked.
	if rl.Allow("client1") {
		t.Error("should block after exceeding burst")
	}
}

func TestRateLimiter_PerClientIsolation(t *testing.T) {
	rl := NewRateLimiter(1, 2)

	// Exhaust client1's budget.
	rl.Allow("client1")
	rl.Allow("client1")

	// client2 should still be allowed.
	if !rl.Allow("client2") {
		t.Error("client2 should not be affected by client1's rate limit")
	}
}

func TestWithBodyLimit(t *testing.T) {
	s := &Server{config: Config{MaxBodyBytes: 10}}
	handler := s.withBodyLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 100)
		_, err := r.Body.Read(buf)
		if err != nil && err.Error() != "EOF" {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Small body should pass.
	req := httptest.NewRequest("POST", "/test", bytes.NewReader([]byte("small")))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("small body: expected 200, got %d", w.Code)
	}
}

func TestWithRateLimit(t *testing.T) {
	s := &Server{
		config:  Config{RateLimit: 1, RateBurst: 1},
		limiter: NewRateLimiter(1, 1),
	}
	handler := s.withRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request allowed.
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("first request: expected 200, got %d", w.Code)
	}

	// Second request should be rate limited.
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected 429, got %d", w2.Code)
	}
}

func TestRateLimiter_RefillsTokens(t *testing.T) {
	rl := NewRateLimiter(1000, 1) // High rate so refill happens quickly.

	rl.Allow("client1") // Use the one token.

	time.Sleep(10 * time.Millisecond)

	// After some time, tokens should refill.
	if !rl.Allow("client1") {
		t.Error("tokens should refill over time")
	}
}
