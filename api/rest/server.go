// Package rest provides the HTTP REST API server for the Kronveil agent.
package rest

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kronveil/kronveil/core/engine"
	"github.com/kronveil/kronveil/intelligence/anomaly"
	"github.com/kronveil/kronveil/intelligence/incident"
)

// Config holds REST API server configuration.
type Config struct {
	Port           int      `yaml:"port" json:"port"`
	APIKey         string   `yaml:"api_key" json:"api_key"`
	AllowedOrigins []string `yaml:"allowed_origins" json:"allowed_origins"`
	MaxBodyBytes   int64    `yaml:"max_body_bytes" json:"max_body_bytes"`
	RateLimit      float64  `yaml:"rate_limit" json:"rate_limit"`
	RateBurst      int      `yaml:"rate_burst" json:"rate_burst"`
	TLSCertFile    string   `yaml:"tls_cert_file" json:"tls_cert_file"`
	TLSKeyFile     string   `yaml:"tls_key_file" json:"tls_key_file"`
	TLSCAFile      string   `yaml:"tls_ca_file" json:"tls_ca_file"`
	MutualTLS      bool     `yaml:"mutual_tls" json:"mutual_tls"`
}

// Server is the REST API server for Kronveil.
type Server struct {
	config    Config
	engine    *engine.Engine
	responder *incident.Responder
	detector  *anomaly.Detector
	server    *http.Server
	mux       *http.ServeMux
	limiter   *RateLimiter
	startErr  chan error
}

// NewServer creates a new REST API server.
func NewServer(config Config, eng *engine.Engine, resp *incident.Responder, det *anomaly.Detector) *Server {
	s := &Server{
		config:    config,
		engine:    eng,
		responder: resp,
		detector:  det,
		mux:       http.NewServeMux(),
	}

	// Apply safe defaults.
	if s.config.MaxBodyBytes <= 0 {
		s.config.MaxBodyBytes = 1 << 20 // 1MB
	}
	if s.config.RateLimit <= 0 {
		s.config.RateLimit = 100 // 100 req/s
	}
	if s.config.RateBurst <= 0 {
		s.config.RateBurst = 200
	}
	s.limiter = NewRateLimiter(s.config.RateLimit, s.config.RateBurst)

	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/v1/health", s.handleHealth)
	s.mux.HandleFunc("/api/v1/status", s.handleStatus)
	s.mux.HandleFunc("/api/v1/incidents", s.withAuth(s.handleIncidents))
	s.mux.HandleFunc("/api/v1/incidents/", s.withAuth(s.handleIncidentByID))
	s.mux.HandleFunc("/api/v1/anomalies", s.withAuth(s.handleListAnomalies))
	s.mux.HandleFunc("/api/v1/collectors", s.withAuth(s.handleListCollectors))
	s.mux.HandleFunc("/api/v1/metrics/summary", s.withAuth(s.handleMetricsSummary))
	s.mux.HandleFunc("/api/v1/test/inject", s.withAuth(s.handleTestInject))
}

// handleIncidents dispatches /api/v1/incidents requests.
func (s *Server) handleIncidents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	s.handleListIncidents(w, r)
}

// handleIncidentByID dispatches /api/v1/incidents/{id}... requests.
func (s *Server) handleIncidentByID(w http.ResponseWriter, r *http.Request) {
	// Parse: /api/v1/incidents/{id} or /api/v1/incidents/{id}/acknowledge etc.
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch {
	case r.Method == http.MethodGet && action == "":
		s.handleGetIncident(w, r, id)
	case r.Method == http.MethodPost && action == "acknowledge":
		s.handleAckIncident(w, r, id)
	case r.Method == http.MethodPost && action == "resolve":
		s.handleResolveIncident(w, r, id)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

// Start begins serving the REST API.
func (s *Server) Start() error {
	handler := s.withCORS(s.withRateLimit(s.withBodyLimit(s.withLogging(s.mux))))

	s.server = &http.Server{
		Addr:           fmt.Sprintf(":%d", s.config.Port),
		Handler:        handler,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// Configure TLS if cert/key provided.
	if s.config.TLSCertFile != "" && s.config.TLSKeyFile != "" {
		tlsConfig, err := buildTLSConfig(s.config.TLSCertFile, s.config.TLSKeyFile, s.config.TLSCAFile, s.config.MutualTLS)
		if err != nil {
			return fmt.Errorf("failed to configure TLS: %w", err)
		}
		s.server.TLSConfig = tlsConfig
		log.Printf("[api] REST API server listening on :%d (TLS enabled, mTLS: %v)", s.config.Port, s.config.MutualTLS)

		s.startErr = make(chan error, 1)
		go func() {
			if err := s.server.ListenAndServeTLS(s.config.TLSCertFile, s.config.TLSKeyFile); err != http.ErrServerClosed {
				log.Printf("[api] REST server error: %v", err)
				s.startErr <- err
			}
			close(s.startErr)
		}()
		return nil
	}

	log.Printf("[api] REST API server listening on :%d", s.config.Port)

	s.startErr = make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("[api] REST server error: %v", err)
			s.startErr <- err
		}
		close(s.startErr)
	}()

	return nil
}

// buildTLSConfig creates a TLS configuration with optional mTLS support.
func buildTLSConfig(certFile, keyFile, caFile string, mutualTLS bool) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}

	if mutualTLS && caFile != "" {
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file %s: %w", caFile, err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.ClientCAs = caCertPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig, nil
}

// StartErr returns a channel that receives any server startup error.
func (s *Server) StartErr() <-chan error {
	return s.startErr
}

// Stop gracefully shuts down the REST API server.
func (s *Server) Stop() error {
	if s.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}
