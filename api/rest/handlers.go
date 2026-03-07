package rest

import (
	"encoding/json"
	"net/http"
)

type apiResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiResponse{Data: data})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiResponse{Error: msg})
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
