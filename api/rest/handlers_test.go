package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kronveil/kronveil/core/engine"
	"github.com/kronveil/kronveil/intelligence/anomaly"
	"github.com/kronveil/kronveil/intelligence/incident"
)

// newTestApp sets up a Server with running engine, detector, and responder.
func newTestApp() *Server {
	reg := engine.NewRegistry()
	eng := engine.NewEngine(reg, nil, nil)
	_ = eng.Start(context.Background())

	det := anomaly.New(anomaly.DefaultConfig())
	_ = det.Start(context.Background())

	cfg := incident.DefaultConfig()
	cfg.AutoRemediate = false
	resp := incident.New(cfg, nil, nil)
	_ = resp.Start(context.Background())

	return NewServer(Config{}, eng, resp, det)
}

func TestHandleHealth_Running(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	s.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleHealth_NotRunning(t *testing.T) {
	s := &Server{engine: nil}

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	s.handleHealth(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleStatus_Running(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()
	s.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleStatus_NilEngine(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()
	s.handleStatus(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleListIncidents_Empty(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("GET", "/api/v1/incidents", nil)
	w := httptest.NewRecorder()
	s.handleListIncidents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleListIncidents_NilResponder(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest("GET", "/api/v1/incidents", nil)
	w := httptest.NewRecorder()
	s.handleListIncidents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleIncidents_MethodNotAllowed(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("POST", "/api/v1/incidents", nil)
	w := httptest.NewRecorder()
	s.handleIncidents(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleGetIncident_NotFound(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("GET", "/api/v1/incidents/nonexistent", nil)
	w := httptest.NewRecorder()
	s.handleGetIncident(w, req, "nonexistent")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleGetIncident_NilResponder(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest("GET", "/api/v1/incidents/INC-0001", nil)
	w := httptest.NewRecorder()
	s.handleGetIncident(w, req, "INC-0001")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleAckIncident_NilResponder(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest("POST", "/api/v1/incidents/INC-0001/acknowledge", nil)
	w := httptest.NewRecorder()
	s.handleAckIncident(w, req, "INC-0001")

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleAckIncident_NotFound(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("POST", "/api/v1/incidents/nonexistent/acknowledge", nil)
	w := httptest.NewRecorder()
	s.handleAckIncident(w, req, "nonexistent")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleResolveIncident_NilResponder(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest("POST", "/api/v1/incidents/INC-0001/resolve", nil)
	w := httptest.NewRecorder()
	s.handleResolveIncident(w, req, "INC-0001")

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleResolveIncident_NotFound(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("POST", "/api/v1/incidents/nonexistent/resolve", nil)
	w := httptest.NewRecorder()
	s.handleResolveIncident(w, req, "nonexistent")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleListAnomalies_Empty(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("GET", "/api/v1/anomalies", nil)
	w := httptest.NewRecorder()
	s.handleListAnomalies(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleListAnomalies_NilDetector(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest("GET", "/api/v1/anomalies", nil)
	w := httptest.NewRecorder()
	s.handleListAnomalies(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleListCollectors_Empty(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("GET", "/api/v1/collectors", nil)
	w := httptest.NewRecorder()
	s.handleListCollectors(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleListCollectors_NilEngine(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest("GET", "/api/v1/collectors", nil)
	w := httptest.NewRecorder()
	s.handleListCollectors(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleMetricsSummary(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("GET", "/api/v1/metrics/summary", nil)
	w := httptest.NewRecorder()
	s.handleMetricsSummary(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp apiResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.Data == nil {
		t.Error("expected data in response")
	}
}

func TestHandleMetricsSummary_NilEngine(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest("GET", "/api/v1/metrics/summary", nil)
	w := httptest.NewRecorder()
	s.handleMetricsSummary(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleTestInject_NotPost(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("GET", "/api/v1/test/inject", nil)
	w := httptest.NewRecorder()
	s.handleTestInject(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleTestInject_ValidEvent(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	event := engine.TelemetryEvent{
		Source:   "test",
		Severity: "critical",
		Payload:  map[string]interface{}{"cpu": 95.0},
	}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest("POST", "/api/v1/test/inject", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.handleTestInject(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", w.Code)
	}
}

func TestHandleTestInject_InvalidBody(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("POST", "/api/v1/test/inject", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	s.handleTestInject(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleTestInjectBurst(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	body, _ := json.Marshal(map[string]string{"source": "test-svc", "signal": "memory"})
	req := httptest.NewRequest("POST", "/api/v1/test/inject?mode=burst", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.handleTestInject(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp apiResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data map in response")
	}
	if data["status"] != "burst_complete" {
		t.Errorf("expected burst_complete status, got %v", data["status"])
	}
}

func TestHandleTestInjectBurst_NilBody(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("POST", "/api/v1/test/inject?mode=burst", nil)
	w := httptest.NewRecorder()
	s.handleTestInject(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleIncidentByID_GetIncident(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	// Inject a critical event to create an incident.
	_ = s.responder.Analyze(context.Background(), &engine.TelemetryEvent{
		ID: "e1", Source: "svc", Severity: engine.SeverityCritical,
		Payload: map[string]interface{}{},
	})
	incidents := s.responder.ListIncidents("")
	if len(incidents) == 0 {
		t.Fatal("expected an incident to be created")
	}
	id := incidents[0].ID

	req := httptest.NewRequest("GET", "/api/v1/incidents/"+id, nil)
	w := httptest.NewRecorder()
	s.handleIncidentByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleIncidentByID_Acknowledge(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	_ = s.responder.Analyze(context.Background(), &engine.TelemetryEvent{
		ID: "e1", Source: "svc", Severity: engine.SeverityCritical,
		Payload: map[string]interface{}{},
	})
	id := s.responder.ListIncidents("")[0].ID

	req := httptest.NewRequest("POST", "/api/v1/incidents/"+id+"/acknowledge", nil)
	w := httptest.NewRecorder()
	s.handleIncidentByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleIncidentByID_Resolve(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	_ = s.responder.Analyze(context.Background(), &engine.TelemetryEvent{
		ID: "e1", Source: "svc", Severity: engine.SeverityCritical,
		Payload: map[string]interface{}{},
	})
	id := s.responder.ListIncidents("")[0].ID

	req := httptest.NewRequest("POST", "/api/v1/incidents/"+id+"/resolve", nil)
	w := httptest.NewRecorder()
	s.handleIncidentByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleIncidentByID_UnknownAction(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("POST", "/api/v1/incidents/INC-0001/unknownaction", nil)
	w := httptest.NewRecorder()
	s.handleIncidentByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestWithLogging(t *testing.T) {
	s := testServer("")
	handler := s.withLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})

	if w.Header().Get("Content-Type") != "application/json" {
		t.Error("expected application/json content type")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleListIncidents_InvalidStatus(t *testing.T) {
	s := newTestApp()
	defer func() { _ = s.engine.Stop() }()

	req := httptest.NewRequest("GET", "/api/v1/incidents?status=invalid", nil)
	w := httptest.NewRecorder()
	s.handleListIncidents(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid status, got %d", w.Code)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "bad request")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp apiResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "bad request" {
		t.Errorf("expected error message 'bad request', got %q", resp.Error)
	}
}
