package rest

import (
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

func TestWithCORS_Headers(t *testing.T) {
	s := testServer("")
	handler := s.withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS Allow-Origin header")
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected CORS Allow-Methods header")
	}
}

func TestWithCORS_OptionsReturns204(t *testing.T) {
	s := testServer("")
	handler := s.withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
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

func TestRateLimiter_RefillsTokens(t *testing.T) {
	rl := NewRateLimiter(1000, 1) // High rate so refill happens quickly.

	rl.Allow("client1") // Use the one token.

	time.Sleep(10 * time.Millisecond)

	// After some time, tokens should refill.
	if !rl.Allow("client1") {
		t.Error("tokens should refill over time")
	}
}
