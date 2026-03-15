package incident

import (
	"context"
	"testing"
	"time"

	"github.com/kronveil/kronveil/core/engine"
	"github.com/kronveil/kronveil/internal/testutil"
)

func newResponder() *Responder {
	return New(DefaultConfig(), nil, nil)
}

func criticalEvent() *engine.TelemetryEvent {
	return &engine.TelemetryEvent{
		ID:        "evt-1",
		Source:    "test-source",
		Severity:  engine.SeverityCritical,
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"type": "cpu_spike"},
	}
}

func TestCreateIncident_CriticalSeverity(t *testing.T) {
	r := newResponder()
	r.config.AutoRemediate = false
	_ = r.Start(context.Background())
	defer func() { _ = r.Stop() }()

	err := r.Analyze(context.Background(), criticalEvent())
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	incidents := r.ListIncidents("")
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].Status != engine.StatusActive {
		t.Errorf("expected active status, got %s", incidents[0].Status)
	}
}

func TestCreateIncident_LowSeverity_Ignored(t *testing.T) {
	r := newResponder()
	_ = r.Start(context.Background())
	defer func() { _ = r.Stop() }()

	event := &engine.TelemetryEvent{
		ID:       "evt-low",
		Source:   "test",
		Severity: engine.SeverityLow,
		Payload:  map[string]interface{}{},
	}
	_ = r.Analyze(context.Background(), event)

	if len(r.ListIncidents("")) != 0 {
		t.Error("low severity events should not create incidents")
	}
}

func TestCorrelation_SameSource(t *testing.T) {
	r := newResponder()
	r.config.AutoRemediate = false
	_ = r.Start(context.Background())
	defer func() { _ = r.Stop() }()

	// First event creates incident.
	_ = r.Analyze(context.Background(), &engine.TelemetryEvent{
		ID:       "evt-1",
		Source:   "same-source",
		Severity: engine.SeverityCritical,
		Payload:  map[string]interface{}{},
	})

	// Second event from same source should correlate.
	_ = r.Analyze(context.Background(), &engine.TelemetryEvent{
		ID:       "evt-2",
		Source:   "same-source",
		Severity: engine.SeverityHigh,
		Payload:  map[string]interface{}{},
	})

	incidents := r.ListIncidents("")
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident (correlated), got %d", len(incidents))
	}
	if len(incidents[0].CorrelatedEvents) != 2 {
		t.Errorf("expected 2 correlated events, got %d", len(incidents[0].CorrelatedEvents))
	}
}

func TestAcknowledgeIncident(t *testing.T) {
	r := newResponder()
	r.config.AutoRemediate = false
	_ = r.Start(context.Background())
	defer func() { _ = r.Stop() }()

	_ = r.Analyze(context.Background(), criticalEvent())
	incidents := r.ListIncidents("")
	id := incidents[0].ID

	if err := r.AcknowledgeIncident(id); err != nil {
		t.Fatalf("AcknowledgeIncident failed: %v", err)
	}

	inc, ok := r.GetIncident(id)
	if !ok {
		t.Fatal("incident not found after acknowledge")
	}
	if inc.Status != engine.StatusAcknowledged {
		t.Errorf("expected acknowledged status, got %s", inc.Status)
	}
	if inc.AcknowledgedAt == nil {
		t.Error("AcknowledgedAt should be set")
	}
}

func TestAcknowledgeIncident_NotFound(t *testing.T) {
	r := newResponder()
	if err := r.AcknowledgeIncident("nonexistent"); err == nil {
		t.Error("expected error for nonexistent incident")
	}
}

func TestResolveIncident(t *testing.T) {
	r := newResponder()
	r.config.AutoRemediate = false
	m := &testutil.MockMetricsRecorder{}
	r.SetMetrics(m)
	_ = r.Start(context.Background())
	defer func() { _ = r.Stop() }()

	_ = r.Analyze(context.Background(), criticalEvent())
	incidents := r.ListIncidents("")
	id := incidents[0].ID

	// Small delay so MTTR is measurable.
	time.Sleep(10 * time.Millisecond)

	if err := r.ResolveIncident(id); err != nil {
		t.Fatalf("ResolveIncident failed: %v", err)
	}

	inc, _ := r.GetIncident(id)
	if inc.Status != engine.StatusResolved {
		t.Errorf("expected resolved status, got %s", inc.Status)
	}
	if inc.ResolvedAt == nil {
		t.Error("ResolvedAt should be set")
	}
	if inc.MTTR == nil || *inc.MTTR <= 0 {
		t.Error("MTTR should be set and positive")
	}
	if m.IncidentsResolved.Load() != 1 {
		t.Errorf("expected 1 incident resolved metric, got %d", m.IncidentsResolved.Load())
	}
}

func TestListIncidents_FilterByStatus(t *testing.T) {
	r := newResponder()
	r.config.AutoRemediate = false
	_ = r.Start(context.Background())
	defer func() { _ = r.Stop() }()

	// Create two incidents.
	_ = r.Analyze(context.Background(), &engine.TelemetryEvent{
		ID: "e1", Source: "s1", Severity: engine.SeverityCritical, Payload: map[string]interface{}{},
	})
	_ = r.Analyze(context.Background(), &engine.TelemetryEvent{
		ID: "e2", Source: "s2", Severity: engine.SeverityCritical, Payload: map[string]interface{}{},
	})

	// Resolve one.
	incidents := r.ListIncidents("")
	_ = r.ResolveIncident(incidents[0].ID)

	active := r.ListIncidents(engine.StatusActive)
	resolved := r.ListIncidents(engine.StatusResolved)

	if len(active) != 1 {
		t.Errorf("expected 1 active incident, got %d", len(active))
	}
	if len(resolved) != 1 {
		t.Errorf("expected 1 resolved incident, got %d", len(resolved))
	}
}

func TestGetIncident(t *testing.T) {
	r := newResponder()
	r.config.AutoRemediate = false
	_ = r.Start(context.Background())
	defer func() { _ = r.Stop() }()

	_ = r.Analyze(context.Background(), criticalEvent())
	incidents := r.ListIncidents("")

	inc, ok := r.GetIncident(incidents[0].ID)
	if !ok {
		t.Error("GetIncident should find existing incident")
	}
	if inc.ID != incidents[0].ID {
		t.Error("GetIncident returned wrong incident")
	}

	_, ok = r.GetIncident("nonexistent")
	if ok {
		t.Error("GetIncident should return false for unknown ID")
	}
}

func TestMetrics_IncidentCreated(t *testing.T) {
	r := newResponder()
	r.config.AutoRemediate = false
	m := &testutil.MockMetricsRecorder{}
	r.SetMetrics(m)
	_ = r.Start(context.Background())
	defer func() { _ = r.Stop() }()

	_ = r.Analyze(context.Background(), criticalEvent())

	if m.IncidentsCreated.Load() != 1 {
		t.Errorf("expected 1 incident created metric, got %d", m.IncidentsCreated.Load())
	}
}

func TestHealth(t *testing.T) {
	r := newResponder()
	r.config.AutoRemediate = false
	_ = r.Start(context.Background())
	defer func() { _ = r.Stop() }()

	h := r.Health()
	if h.Name != "incident-responder" {
		t.Errorf("expected name 'incident-responder', got %s", h.Name)
	}
	if h.Status != "healthy" {
		t.Errorf("expected healthy status, got %s", h.Status)
	}

	// Create an incident to check active count.
	_ = r.Analyze(context.Background(), criticalEvent())
	h = r.Health()
	if h.Message == "" {
		t.Error("expected health message with active incident count")
	}
}

func TestIncidentsChannel(t *testing.T) {
	r := newResponder()
	r.config.AutoRemediate = false
	_ = r.Start(context.Background())

	ch := r.Incidents()
	if ch == nil {
		t.Fatal("Incidents channel should not be nil")
	}

	_ = r.Analyze(context.Background(), criticalEvent())

	select {
	case inc := <-ch:
		if inc == nil {
			t.Error("expected non-nil incident on channel")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected incident on channel")
	}

	_ = r.Stop()
}

func TestAutoRemediation_DryRun(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoRemediate = true
	cfg.DryRun = true
	notifier := &testutil.MockNotifier{}
	r := New(cfg, []engine.Notifier{notifier}, nil)
	m := &testutil.MockMetricsRecorder{}
	r.SetMetrics(m)
	_ = r.Start(context.Background())
	defer func() { _ = r.Stop() }()

	// High-severity event triggers remediation.
	_ = r.Analyze(context.Background(), &engine.TelemetryEvent{
		ID:       "evt-high",
		Source:   "app-server",
		Severity: engine.SeverityHigh,
		Payload:  map[string]interface{}{},
	})

	// Give goroutine time to run remediation.
	time.Sleep(200 * time.Millisecond)

	if m.RemediationsOK.Load() == 0 && m.RemediationsFail.Load() == 0 {
		t.Error("expected remediation metric to be recorded")
	}
}

func TestNotifier_CalledOnIncident(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoRemediate = false
	notifier := &testutil.MockNotifier{}
	r := New(cfg, []engine.Notifier{notifier}, nil)
	_ = r.Start(context.Background())
	defer func() { _ = r.Stop() }()

	_ = r.Analyze(context.Background(), criticalEvent())

	// Give notification time to complete.
	time.Sleep(50 * time.Millisecond)

	if notifier.IncidentCount() == 0 {
		t.Error("expected notifier to be called with incident")
	}
}

func TestCreateIncident_WithEventType(t *testing.T) {
	r := newResponder()
	r.config.AutoRemediate = false
	_ = r.Start(context.Background())
	defer func() { _ = r.Stop() }()

	// Event with "type" in payload should use it as title.
	_ = r.Analyze(context.Background(), &engine.TelemetryEvent{
		ID:       "evt-typed",
		Source:   "test",
		Severity: engine.SeverityCritical,
		Payload:  map[string]interface{}{"type": "OOMKilled"},
	})

	incidents := r.ListIncidents("")
	if len(incidents) != 1 {
		t.Fatal("expected 1 incident")
	}
	if incidents[0].Title != "OOMKilled" {
		t.Errorf("expected title 'OOMKilled', got %s", incidents[0].Title)
	}
}

func TestCreateIncident_WithoutEventType(t *testing.T) {
	r := newResponder()
	r.config.AutoRemediate = false
	_ = r.Start(context.Background())
	defer func() { _ = r.Stop() }()

	_ = r.Analyze(context.Background(), &engine.TelemetryEvent{
		ID:       "evt-no-type",
		Source:   "test-src",
		Severity: engine.SeverityHigh,
		Payload:  map[string]interface{}{"cpu": 95.0},
	})

	incidents := r.ListIncidents("")
	if len(incidents) != 1 {
		t.Fatal("expected 1 incident")
	}
	// Title should contain severity and source.
	if incidents[0].Title == "" {
		t.Error("expected non-empty title")
	}
}

func TestName(t *testing.T) {
	r := newResponder()
	if r.Name() != "incident-responder" {
		t.Errorf("expected 'incident-responder', got %s", r.Name())
	}
}
