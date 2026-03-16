// Package testutil provides test helpers and mock implementations.
package testutil

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// MockCollector implements engine.Collector for testing.
type MockCollector struct {
	NameVal  string
	EventsCh chan *engine.TelemetryEvent
	Started  bool
	Stopped  bool
	HealthFn func() engine.ComponentHealth
}

func NewMockCollector(name string) *MockCollector {
	return &MockCollector{
		NameVal:  name,
		EventsCh: make(chan *engine.TelemetryEvent, 100),
	}
}

func (m *MockCollector) Name() string { return m.NameVal }
func (m *MockCollector) Start(_ context.Context) error {
	m.Started = true
	return nil
}
func (m *MockCollector) Stop() error {
	m.Stopped = true
	close(m.EventsCh)
	return nil
}
func (m *MockCollector) Events() <-chan *engine.TelemetryEvent { return m.EventsCh }
func (m *MockCollector) Health() engine.ComponentHealth {
	if m.HealthFn != nil {
		return m.HealthFn()
	}
	return engine.ComponentHealth{
		Name:      m.NameVal,
		Status:    "healthy",
		Message:   "mock collector",
		LastCheck: time.Now(),
	}
}

// MockModule implements engine.IntelligenceModule for testing.
type MockModule struct {
	NameVal    string
	Started    bool
	Stopped    bool
	mu         sync.Mutex
	AnalyzedEvents []*engine.TelemetryEvent
	AnalyzeErr error
	HealthFn   func() engine.ComponentHealth
}

func NewMockModule(name string) *MockModule {
	return &MockModule{NameVal: name}
}

func (m *MockModule) Name() string { return m.NameVal }
func (m *MockModule) Start(_ context.Context) error {
	m.Started = true
	return nil
}
func (m *MockModule) Stop() error {
	m.Stopped = true
	return nil
}
func (m *MockModule) Analyze(_ context.Context, event *engine.TelemetryEvent) error {
	m.mu.Lock()
	m.AnalyzedEvents = append(m.AnalyzedEvents, event)
	m.mu.Unlock()
	return m.AnalyzeErr
}
func (m *MockModule) Events() []*engine.TelemetryEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*engine.TelemetryEvent, len(m.AnalyzedEvents))
	copy(result, m.AnalyzedEvents)
	return result
}
func (m *MockModule) Health() engine.ComponentHealth {
	if m.HealthFn != nil {
		return m.HealthFn()
	}
	return engine.ComponentHealth{
		Name:      m.NameVal,
		Status:    "healthy",
		Message:   "mock module",
		LastCheck: time.Now(),
	}
}

// MockIntegration implements engine.Integration for testing.
type MockIntegration struct {
	NameVal     string
	Initialized bool
	Closed      bool
	InitErr     error
	HealthFn    func() engine.ComponentHealth
}

func NewMockIntegration(name string) *MockIntegration {
	return &MockIntegration{NameVal: name}
}

func (m *MockIntegration) Name() string { return m.NameVal }
func (m *MockIntegration) Initialize(_ context.Context) error {
	m.Initialized = true
	return m.InitErr
}
func (m *MockIntegration) Close() error {
	m.Closed = true
	return nil
}
func (m *MockIntegration) Health() engine.ComponentHealth {
	if m.HealthFn != nil {
		return m.HealthFn()
	}
	return engine.ComponentHealth{
		Name:      m.NameVal,
		Status:    "healthy",
		Message:   "mock integration",
		LastCheck: time.Now(),
	}
}

// MockNotifier implements engine.Notifier for testing.
type MockNotifier struct {
	mu          sync.Mutex
	Incidents   []*engine.Incident
	Anomalies   []*engine.Anomaly
	Remediations []*engine.RemediationAction
}

func (m *MockNotifier) NotifyIncident(_ context.Context, inc *engine.Incident) error {
	m.mu.Lock()
	m.Incidents = append(m.Incidents, inc)
	m.mu.Unlock()
	return nil
}

func (m *MockNotifier) NotifyAnomaly(_ context.Context, a *engine.Anomaly) error {
	m.mu.Lock()
	m.Anomalies = append(m.Anomalies, a)
	m.mu.Unlock()
	return nil
}

func (m *MockNotifier) NotifyRemediation(_ context.Context, action *engine.RemediationAction) error {
	m.mu.Lock()
	m.Remediations = append(m.Remediations, action)
	m.mu.Unlock()
	return nil
}

// IncidentCount returns the number of incidents notified (thread-safe).
func (m *MockNotifier) IncidentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Incidents)
}

// MockMetricsRecorder implements engine.MetricsRecorder with atomic counters.
type MockMetricsRecorder struct {
	Events            atomic.Int64
	CollectorErrors   atomic.Int64
	Anomalies         atomic.Int64
	IncidentsCreated  atomic.Int64
	IncidentsResolved atomic.Int64
	RemediationsOK    atomic.Int64
	RemediationsFail  atomic.Int64
	PolicyEvals       atomic.Int64
	PolicyViolations  atomic.Int64
	MTTRVal           atomic.Int64 // stores float64 bits
	HealthyVal        atomic.Int64
	DegradedVal       atomic.Int64
	CriticalVal       atomic.Int64
	UptimeVal         atomic.Int64 // stores float64 bits
}

func (m *MockMetricsRecorder) RecordEvent(string)          { m.Events.Add(1) }
func (m *MockMetricsRecorder) RecordCollectorError(string)  { m.CollectorErrors.Add(1) }
func (m *MockMetricsRecorder) RecordAnomaly()               { m.Anomalies.Add(1) }
func (m *MockMetricsRecorder) RecordIncidentCreated()       { m.IncidentsCreated.Add(1) }
func (m *MockMetricsRecorder) RecordIncidentResolved()      { m.IncidentsResolved.Add(1) }
func (m *MockMetricsRecorder) RecordRemediation(success bool) {
	if success {
		m.RemediationsOK.Add(1)
	} else {
		m.RemediationsFail.Add(1)
	}
}
func (m *MockMetricsRecorder) SetMTTR(seconds float64) {
	m.MTTRVal.Store(int64(seconds * 1000)) // store as millis for easy comparison
}
func (m *MockMetricsRecorder) RecordPolicyEvaluation() { m.PolicyEvals.Add(1) }
func (m *MockMetricsRecorder) RecordPolicyViolation()  { m.PolicyViolations.Add(1) }
func (m *MockMetricsRecorder) SetComponentHealth(healthy, degraded, critical int) {
	m.HealthyVal.Store(int64(healthy))
	m.DegradedVal.Store(int64(degraded))
	m.CriticalVal.Store(int64(critical))
}
func (m *MockMetricsRecorder) SetUptime(seconds float64) {
	m.UptimeVal.Store(int64(seconds * 1000))
}

// MockLLMProvider implements engine.LLMProvider for testing.
type MockLLMProvider struct {
	Response string
	Err      error
}

func (m *MockLLMProvider) Invoke(_ context.Context, _ string) (string, error) {
	return m.Response, m.Err
}

func (m *MockLLMProvider) InvokeWithSystem(_ context.Context, _, _ string) (string, error) {
	return m.Response, m.Err
}

// MockPublisher implements engine.EventPublisher for testing.
type MockPublisher struct {
	mu     sync.Mutex
	Events []*engine.TelemetryEvent
	Err    error
}

func (m *MockPublisher) Publish(_ context.Context, _ string, event *engine.TelemetryEvent) error {
	if m.Err != nil {
		return m.Err
	}
	m.mu.Lock()
	m.Events = append(m.Events, event)
	m.mu.Unlock()
	return nil
}

func (m *MockPublisher) Published() []*engine.TelemetryEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*engine.TelemetryEvent, len(m.Events))
	copy(result, m.Events)
	return result
}
