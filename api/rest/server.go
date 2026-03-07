package rest

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/kronveil/kronveil/core/engine"
	"github.com/kronveil/kronveil/intelligence/incident"
)

// Config holds REST API server configuration.
type Config struct {
	Port   int    `yaml:"port" json:"port"`
	APIKey string `yaml:"api_key" json:"api_key"`
}

// Server is the REST API server for Kronveil.
type Server struct {
	config    Config
	engine    *engine.Engine
	responder *incident.Responder
	server    *http.Server
	mux       *http.ServeMux
}

// NewServer creates a new REST API server.
func NewServer(config Config, eng *engine.Engine, resp *incident.Responder) *Server {
	s := &Server{
		config:    config,
		engine:    eng,
		responder: resp,
		mux:       http.NewServeMux(),
	}

	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	// Health endpoints
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/status", s.handleStatus)

	// Incident endpoints
	s.mux.HandleFunc("GET /api/v1/incidents", s.withAuth(s.handleListIncidents))
	s.mux.HandleFunc("GET /api/v1/incidents/{id}", s.withAuth(s.handleGetIncident))
	s.mux.HandleFunc("POST /api/v1/incidents/{id}/acknowledge", s.withAuth(s.handleAckIncident))
	s.mux.HandleFunc("POST /api/v1/incidents/{id}/resolve", s.withAuth(s.handleResolveIncident))

	// Anomaly endpoints
	s.mux.HandleFunc("GET /api/v1/anomalies", s.withAuth(s.handleListAnomalies))

	// Collector endpoints
	s.mux.HandleFunc("GET /api/v1/collectors", s.withAuth(s.handleListCollectors))

	// Metrics endpoint
	s.mux.HandleFunc("GET /api/v1/metrics/summary", s.withAuth(s.handleMetricsSummary))
}

// Start begins serving the REST API.
func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      s.withCORS(s.withLogging(s.mux)),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("[api] REST API server listening on :%d", s.config.Port)

	go func() {
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("[api] REST server error: %v", err)
		}
	}()

	return nil
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
