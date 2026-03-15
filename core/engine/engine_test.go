package engine

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// mockPublisher for engine tests (internal to package).
type mockPublisher struct {
	events []*TelemetryEvent
	err    error
}

func (m *mockPublisher) Publish(_ context.Context, _ string, event *TelemetryEvent) error {
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, event)
	return nil
}

// mockMetrics for engine tests.
type mockMetrics struct {
	events    int
	errors    int
	healthy   int
	degraded  int
	critical  int
	uptime    float64
}

func (m *mockMetrics) RecordEvent(string)          { m.events++ }
func (m *mockMetrics) RecordCollectorError(string)  { m.errors++ }
func (m *mockMetrics) RecordAnomaly()               {}
func (m *mockMetrics) RecordIncidentCreated()        {}
func (m *mockMetrics) RecordIncidentResolved()       {}
func (m *mockMetrics) RecordRemediation(bool)        {}
func (m *mockMetrics) SetMTTR(float64)               {}
func (m *mockMetrics) RecordPolicyEvaluation()       {}
func (m *mockMetrics) RecordPolicyViolation()        {}
func (m *mockMetrics) SetComponentHealth(h, d, c int) {
	m.healthy = h
	m.degraded = d
	m.critical = c
}
func (m *mockMetrics) SetUptime(s float64) { m.uptime = s }

func TestNewEngine_NilMetrics(t *testing.T) {
	r := NewRegistry()
	e := NewEngine(r, nil, nil)
	if e.metrics == nil {
		t.Error("metrics should default to NoopMetricsRecorder")
	}
	if _, ok := e.metrics.(*NoopMetricsRecorder); !ok {
		t.Errorf("expected NoopMetricsRecorder, got %T", e.metrics)
	}
}

func TestEngine_StartStop(t *testing.T) {
	r := NewRegistry()
	col := &stubCollector{name: "test-col"}
	mod := &stubModule{name: "test-mod"}
	intg := &stubIntegration{name: "test-intg"}
	_ = r.RegisterCollector(col)
	_ = r.RegisterModule(mod)
	_ = r.RegisterIntegration(intg)

	e := NewEngine(r, nil, nil)
	ctx := context.Background()

	if err := e.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !e.IsRunning() {
		t.Error("engine should be running after Start")
	}

	// Double start should fail.
	if err := e.Start(ctx); err == nil {
		t.Error("expected error for double start")
	}

	if err := e.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if e.IsRunning() {
		t.Error("engine should not be running after Stop")
	}
}

func TestEngine_RouteEvents(t *testing.T) {
	r := NewRegistry()

	eventCh := make(chan *TelemetryEvent, 10)
	col := &channelCollector{name: "test", ch: eventCh}
	mod := &recordingModule{name: "mod"}
	_ = r.RegisterCollector(col)
	_ = r.RegisterModule(mod)

	pub := &mockPublisher{}
	met := &mockMetrics{}
	e := NewEngine(r, pub, met)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := e.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Send an event.
	eventCh <- &TelemetryEvent{
		ID:     "test-event",
		Source: "test",
	}

	// Allow routing to process.
	time.Sleep(50 * time.Millisecond)

	close(eventCh)
	_ = e.Stop()

	if met.events != 1 {
		t.Errorf("expected 1 event recorded, got %d", met.events)
	}
	if len(pub.events) != 1 {
		t.Errorf("expected 1 published event, got %d", len(pub.events))
	}
	if len(mod.analyzed) != 1 {
		t.Errorf("expected 1 analyzed event, got %d", len(mod.analyzed))
	}
}

func TestEngine_Status(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterCollector(&stubCollector{name: "col"})
	_ = r.RegisterIntegration(&stubIntegration{name: "intg", status: "degraded"})

	met := &mockMetrics{}
	e := NewEngine(r, nil, met)

	ctx := context.Background()
	_ = e.Start(ctx)
	defer func() { _ = e.Stop() }()

	time.Sleep(10 * time.Millisecond) // Let uptime accumulate.
	status := e.Status()

	if status.Status != "degraded" {
		t.Errorf("expected degraded status, got %s", status.Status)
	}
	if met.healthy != 1 {
		t.Errorf("expected 1 healthy, got %d", met.healthy)
	}
	if met.degraded != 1 {
		t.Errorf("expected 1 degraded, got %d", met.degraded)
	}
}

func TestEngine_PublisherError(t *testing.T) {
	r := NewRegistry()
	eventCh := make(chan *TelemetryEvent, 10)
	col := &channelCollector{name: "test", ch: eventCh}
	_ = r.RegisterCollector(col)

	pub := &mockPublisher{err: fmt.Errorf("publish error")}
	met := &mockMetrics{}
	e := NewEngine(r, pub, met)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = e.Start(ctx)

	eventCh <- &TelemetryEvent{ID: "e1", Source: "test"}
	time.Sleep(50 * time.Millisecond)

	close(eventCh)
	_ = e.Stop()

	if met.errors != 1 {
		t.Errorf("expected 1 collector error, got %d", met.errors)
	}
}

func TestEngine_Status_Critical(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterIntegration(&stubIntegration{name: "i1", status: "critical"})
	_ = r.RegisterIntegration(&stubIntegration{name: "i2", status: "degraded"})

	met := &mockMetrics{}
	e := NewEngine(r, nil, met)
	_ = e.Start(context.Background())
	defer func() { _ = e.Stop() }()

	status := e.Status()
	if status.Status != "critical" {
		t.Errorf("expected critical status, got %s", status.Status)
	}
	if met.critical != 1 {
		t.Errorf("expected 1 critical, got %d", met.critical)
	}
}

func TestEngine_StopWhenNotRunning(t *testing.T) {
	r := NewRegistry()
	e := NewEngine(r, nil, nil)
	// Stop without Start should be a no-op.
	if err := e.Stop(); err != nil {
		t.Errorf("Stop on not-running engine should succeed: %v", err)
	}
}

// channelCollector allows sending events through a channel for testing.
type channelCollector struct {
	name string
	ch   chan *TelemetryEvent
}

func (c *channelCollector) Name() string                  { return c.name }
func (c *channelCollector) Start(context.Context) error   { return nil }
func (c *channelCollector) Stop() error                   { return nil }
func (c *channelCollector) Events() <-chan *TelemetryEvent { return c.ch }
func (c *channelCollector) Health() ComponentHealth {
	return ComponentHealth{Name: c.name, Status: "healthy", LastCheck: time.Now()}
}

// recordingModule captures analyzed events.
type recordingModule struct {
	name     string
	analyzed []*TelemetryEvent
}

func (m *recordingModule) Name() string                { return m.name }
func (m *recordingModule) Start(context.Context) error { return nil }
func (m *recordingModule) Stop() error                 { return nil }
func (m *recordingModule) Analyze(_ context.Context, event *TelemetryEvent) error {
	m.analyzed = append(m.analyzed, event)
	return nil
}
func (m *recordingModule) Health() ComponentHealth {
	return ComponentHealth{Name: m.name, Status: "healthy", LastCheck: time.Now()}
}
