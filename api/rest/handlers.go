package rest

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

type apiResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{Data: data})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{Error: msg})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil || !s.engine.IsRunning() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "engine not initialized")
		return
	}
	writeJSON(w, http.StatusOK, s.engine.Status())
}

func (s *Server) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	if s.responder == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	status := r.URL.Query().Get("status")
	incidents := s.responder.ListIncidents(status)
	writeJSON(w, http.StatusOK, incidents)
}

func (s *Server) handleGetIncident(w http.ResponseWriter, r *http.Request, id string) {
	if s.responder == nil {
		writeError(w, http.StatusNotFound, "incident not found")
		return
	}
	inc, ok := s.responder.GetIncident(id)
	if !ok {
		writeError(w, http.StatusNotFound, "incident not found")
		return
	}
	writeJSON(w, http.StatusOK, inc)
}

func (s *Server) handleAckIncident(w http.ResponseWriter, r *http.Request, id string) {
	if s.responder == nil {
		writeError(w, http.StatusNotFound, "incident not found")
		return
	}
	if err := s.responder.AcknowledgeIncident(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}

func (s *Server) handleResolveIncident(w http.ResponseWriter, r *http.Request, id string) {
	if s.responder == nil {
		writeError(w, http.StatusNotFound, "incident not found")
		return
	}
	if err := s.responder.ResolveIncident(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

func (s *Server) handleListAnomalies(w http.ResponseWriter, r *http.Request) {
	if s.detector != nil {
		writeJSON(w, http.StatusOK, s.detector.ListAnomalies())
		return
	}
	writeJSON(w, http.StatusOK, []interface{}{})
}

func (s *Server) handleListCollectors(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	collectors := s.engine.Registry().Collectors()
	var result []map[string]interface{}
	for _, c := range collectors {
		h := c.Health()
		result = append(result, map[string]interface{}{
			"name":    c.Name(),
			"status":  h.Status,
			"message": h.Message,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleTestInject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	mode := r.URL.Query().Get("mode")

	if mode == "burst" {
		s.handleTestInjectBurst(w, r)
		return
	}

	var event engine.TelemetryEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid event: "+err.Error())
		return
	}

	if event.ID == "" {
		event.ID = fmt.Sprintf("test-%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Source == "" {
		event.Source = "test"
	}

	for _, mod := range s.engine.Registry().Modules() {
		if err := mod.Analyze(r.Context(), &event); err != nil {
			log.Printf("[api] Module %s analysis error: %v", mod.Name(), err)
		}
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":   "injected",
		"event_id": event.ID,
	})
}

func (s *Server) handleTestInjectBurst(w http.ResponseWriter, r *http.Request) {
	source := "test-app"
	signal := "cpu_usage"

	var req struct {
		Source string `json:"source"`
		Signal string `json:"signal"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Source != "" {
		source = req.Source
	}
	if req.Signal != "" {
		signal = req.Signal
	}

	injected := 0
	// Seed with 35 normal events (values around 50 with small variance).
	for i := 0; i < 35; i++ {
		event := &engine.TelemetryEvent{
			ID:        fmt.Sprintf("burst-%d-%d", i, time.Now().UnixNano()),
			Source:    source,
			Type:      "metric",
			Timestamp: time.Now(),
			Severity:  engine.SeverityInfo,
			Payload:   map[string]interface{}{signal: 50.0 + rand.Float64()*4.0 - 2.0},
		}
		for _, mod := range s.engine.Registry().Modules() {
			_ = mod.Analyze(r.Context(), event)
		}
		injected++
	}

	// Inject spike event (value ~200, well above 3 sigma from mean of ~50).
	spikeEvent := &engine.TelemetryEvent{
		ID:        fmt.Sprintf("burst-spike-%d", time.Now().UnixNano()),
		Source:    source,
		Type:      "metric",
		Timestamp: time.Now(),
		Severity:  engine.SeverityCritical,
		Payload:   map[string]interface{}{signal: 200.0},
	}
	for _, mod := range s.engine.Registry().Modules() {
		_ = mod.Analyze(r.Context(), spikeEvent)
	}
	injected++

	// Collect results.
	var anomalies []*engine.Anomaly
	if s.detector != nil {
		anomalies = s.detector.ListAnomalies()
	}
	var incidents []*engine.Incident
	if s.responder != nil {
		incidents = s.responder.ListIncidents("")
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":          "burst_complete",
		"events_injected": injected,
		"anomalies_found": len(anomalies),
		"incidents_created": len(incidents),
		"anomalies":       anomalies,
		"incidents":       incidents,
	})
}

func (s *Server) handleMetricsSummary(w http.ResponseWriter, r *http.Request) {
	summary := map[string]interface{}{
		"events_total":     0,
		"events_per_sec":   0,
		"active_incidents": 0,
		"anomalies_24h":    0,
		"mttr_avg_sec":     0,
		"collectors":       0,
	}
	if s.engine != nil {
		summary["collectors"] = len(s.engine.Registry().Collectors())
	}
	writeJSON(w, http.StatusOK, summary)
}
