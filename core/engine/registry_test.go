package engine

import (
	"context"
	"testing"
	"time"
)

// stubCollector is a minimal Collector for registry tests.
type stubCollector struct {
	name string
}

func (s *stubCollector) Name() string                              { return s.name }
func (s *stubCollector) Start(context.Context) error               { return nil }
func (s *stubCollector) Stop() error                               { return nil }
func (s *stubCollector) Events() <-chan *TelemetryEvent {
	ch := make(chan *TelemetryEvent)
	close(ch)
	return ch
}
func (s *stubCollector) Health() ComponentHealth {
	return ComponentHealth{Name: s.name, Status: "healthy", LastCheck: time.Now()}
}

// stubModule is a minimal IntelligenceModule for registry tests.
type stubModule struct {
	name string
}

func (s *stubModule) Name() string                                  { return s.name }
func (s *stubModule) Start(context.Context) error                   { return nil }
func (s *stubModule) Stop() error                                   { return nil }
func (s *stubModule) Analyze(context.Context, *TelemetryEvent) error { return nil }
func (s *stubModule) Health() ComponentHealth {
	return ComponentHealth{Name: s.name, Status: "healthy", LastCheck: time.Now()}
}

// stubIntegration is a minimal Integration for registry tests.
type stubIntegration struct {
	name   string
	status string
}

func (s *stubIntegration) Name() string                { return s.name }
func (s *stubIntegration) Initialize(context.Context) error { return nil }
func (s *stubIntegration) Close() error                { return nil }
func (s *stubIntegration) Health() ComponentHealth {
	st := s.status
	if st == "" {
		st = "healthy"
	}
	return ComponentHealth{Name: s.name, Status: st, LastCheck: time.Now()}
}

func TestRegisterCollector(t *testing.T) {
	r := NewRegistry()
	c := &stubCollector{name: "test-col"}

	if err := r.RegisterCollector(c); err != nil {
		t.Fatalf("RegisterCollector failed: %v", err)
	}

	// Duplicate should error.
	if err := r.RegisterCollector(c); err == nil {
		t.Error("expected error for duplicate collector")
	}
}

func TestRegisterModule(t *testing.T) {
	r := NewRegistry()
	m := &stubModule{name: "test-mod"}

	if err := r.RegisterModule(m); err != nil {
		t.Fatalf("RegisterModule failed: %v", err)
	}

	if err := r.RegisterModule(m); err == nil {
		t.Error("expected error for duplicate module")
	}
}

func TestRegisterIntegration(t *testing.T) {
	r := NewRegistry()
	i := &stubIntegration{name: "test-intg"}

	if err := r.RegisterIntegration(i); err != nil {
		t.Fatalf("RegisterIntegration failed: %v", err)
	}

	if err := r.RegisterIntegration(i); err == nil {
		t.Error("expected error for duplicate integration")
	}
}

func TestGetCollector(t *testing.T) {
	r := NewRegistry()
	c := &stubCollector{name: "k8s"}
	_ = r.RegisterCollector(c)

	got, ok := r.GetCollector("k8s")
	if !ok || got.Name() != "k8s" {
		t.Error("GetCollector should find registered collector")
	}

	_, ok = r.GetCollector("nonexistent")
	if ok {
		t.Error("GetCollector should return false for unknown")
	}
}

func TestGetModule(t *testing.T) {
	r := NewRegistry()
	m := &stubModule{name: "anomaly"}
	_ = r.RegisterModule(m)

	got, ok := r.GetModule("anomaly")
	if !ok || got.Name() != "anomaly" {
		t.Error("GetModule should find registered module")
	}

	_, ok = r.GetModule("nonexistent")
	if ok {
		t.Error("GetModule should return false for unknown")
	}
}

func TestHealth_Aggregation(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterCollector(&stubCollector{name: "col1"})
	_ = r.RegisterModule(&stubModule{name: "mod1"})
	_ = r.RegisterIntegration(&stubIntegration{name: "intg1", status: "degraded"})

	health := r.Health()
	if len(health) != 3 {
		t.Fatalf("expected 3 health entries, got %d", len(health))
	}

	found := map[string]bool{}
	for _, h := range health {
		found[h.Name] = true
	}
	for _, name := range []string{"col1", "mod1", "intg1"} {
		if !found[name] {
			t.Errorf("missing health entry for %s", name)
		}
	}
}

func TestCollectorsModulesIntegrations_List(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterCollector(&stubCollector{name: "c1"})
	_ = r.RegisterCollector(&stubCollector{name: "c2"})
	_ = r.RegisterModule(&stubModule{name: "m1"})
	_ = r.RegisterIntegration(&stubIntegration{name: "i1"})

	if len(r.Collectors()) != 2 {
		t.Errorf("expected 2 collectors, got %d", len(r.Collectors()))
	}
	if len(r.Modules()) != 1 {
		t.Errorf("expected 1 module, got %d", len(r.Modules()))
	}
	if len(r.Integrations()) != 1 {
		t.Errorf("expected 1 integration, got %d", len(r.Integrations()))
	}
}
