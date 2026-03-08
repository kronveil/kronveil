package rest

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/kronveil/kronveil/core/engine"
	"github.com/kronveil/kronveil/intelligence/anomaly"
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
	detector  *anomaly.Detector
	server    *http.Server
	mux       *http.ServeMux
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
