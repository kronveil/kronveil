package rootcause

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kronveil/kronveil/core/engine"
	"github.com/kronveil/kronveil/internal/testutil"
)

func TestHeuristicAnalysis_SingleService(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode(&ServiceNode{Name: "api"})
	a := New(g, nil)

	events := []engine.TelemetryEvent{{
		Source:   "api",
		Severity: engine.SeverityCritical,
		Payload:  map[string]interface{}{},
	}}

	rc, err := a.AnalyzeIncident(context.Background(), "api", events)
	if err != nil {
		t.Fatalf("AnalyzeIncident failed: %v", err)
	}
	if rc.Confidence == 0 {
		t.Error("expected non-zero confidence")
	}
	if rc.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestHeuristicAnalysis_ConfidenceVariesByEvidence(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode(&ServiceNode{Name: "api"})
	a := New(g, nil)

	// Few events → lower confidence.
	fewEvents := []engine.TelemetryEvent{{Source: "api", Severity: engine.SeverityHigh}}
	rc1, _ := a.AnalyzeIncident(context.Background(), "api", fewEvents)

	// More events → higher confidence.
	manyEvents := make([]engine.TelemetryEvent, 5)
	for i := range manyEvents {
		manyEvents[i] = engine.TelemetryEvent{Source: "api", Severity: engine.SeverityHigh}
	}
	rc2, _ := a.AnalyzeIncident(context.Background(), "api", manyEvents)

	if rc2.Confidence <= rc1.Confidence {
		t.Errorf("more evidence should increase confidence: few=%f, many=%f", rc1.Confidence, rc2.Confidence)
	}
}

func TestAnalyzeIncident_NoLLM_FallsBackToHeuristic(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode(&ServiceNode{Name: "svc"})
	a := New(g, nil) // nil LLM

	rc, err := a.AnalyzeIncident(context.Background(), "svc", []engine.TelemetryEvent{
		{Source: "svc", Severity: engine.SeverityCritical},
	})
	if err != nil {
		t.Fatalf("expected heuristic fallback, got error: %v", err)
	}
	if rc == nil {
		t.Fatal("expected non-nil root cause from heuristic")
	}
}

func TestAnalyzeIncident_WithLLM(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode(&ServiceNode{Name: "svc"})
	llm := &testutil.MockLLMProvider{Response: "Database connection pool exhausted"}
	a := New(g, llm)

	rc, err := a.AnalyzeIncident(context.Background(), "svc", []engine.TelemetryEvent{
		{Source: "svc", Severity: engine.SeverityCritical},
	})
	if err != nil {
		t.Fatalf("AnalyzeIncident failed: %v", err)
	}
	if rc.Summary != "Database connection pool exhausted" {
		t.Errorf("unexpected summary: %s", rc.Summary)
	}
	if rc.Confidence != 0.85 {
		t.Errorf("LLM analysis confidence = %f, want 0.85", rc.Confidence)
	}
}

func TestGetAnalysis_CacheHit(t *testing.T) {
	g := NewDependencyGraph()
	a := New(g, nil)

	// Manually cache an analysis.
	a.cache["evt-123"] = &RootCause{
		Summary:   "cached result",
		Timestamp: time.Now(),
	}

	rc, ok := a.GetAnalysis("evt-123")
	if !ok || rc.Summary != "cached result" {
		t.Error("expected cache hit")
	}
}

func TestGetAnalysis_CacheMiss(t *testing.T) {
	g := NewDependencyGraph()
	a := New(g, nil)

	_, ok := a.GetAnalysis("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestAnalyzeIncident_LLMError_FallsBackToHeuristic(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode(&ServiceNode{Name: "svc"})
	llm := &testutil.MockLLMProvider{Err: fmt.Errorf("LLM unavailable")}
	a := New(g, llm)

	rc, err := a.AnalyzeIncident(context.Background(), "svc", []engine.TelemetryEvent{
		{Source: "svc", Severity: engine.SeverityCritical},
	})
	if err != nil {
		t.Fatalf("expected fallback, got error: %v", err)
	}
	if rc == nil {
		t.Fatal("expected non-nil root cause from heuristic fallback")
	}
	// Heuristic confidence should be less than LLM confidence.
	if rc.Confidence >= 0.85 {
		t.Errorf("expected heuristic confidence (<0.85), got %f", rc.Confidence)
	}
}

func TestAnalyzer_StartStopHealth(t *testing.T) {
	g := NewDependencyGraph()
	a := New(g, nil)

	if a.Name() != "root-cause-analyzer" {
		t.Errorf("expected name 'root-cause-analyzer', got %s", a.Name())
	}

	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	h := a.Health()
	if h.Status != "healthy" {
		t.Errorf("expected healthy, got %s", h.Status)
	}

	if err := a.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestAnalyze_HighSeverityEvent(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode(&ServiceNode{Name: "api"})
	a := New(g, nil)
	_ = a.Start(context.Background())
	defer func() { _ = a.Stop() }()

	event := &engine.TelemetryEvent{
		ID:       "evt-critical",
		Source:   "api",
		Severity: engine.SeverityCritical,
		Payload:  map[string]interface{}{"error": "timeout"},
	}
	err := a.Analyze(context.Background(), event)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Give the goroutine time to cache the result.
	time.Sleep(100 * time.Millisecond)

	rc, ok := a.GetAnalysis("evt-critical")
	if !ok {
		t.Error("expected cached analysis for critical event")
	}
	if rc == nil {
		t.Error("expected non-nil root cause")
	}
}

func TestAnalyze_LowSeverity_Skipped(t *testing.T) {
	g := NewDependencyGraph()
	a := New(g, nil)
	_ = a.Start(context.Background())
	defer func() { _ = a.Stop() }()

	event := &engine.TelemetryEvent{
		ID:       "evt-low",
		Source:   "api",
		Severity: engine.SeverityLow,
	}
	_ = a.Analyze(context.Background(), event)

	time.Sleep(50 * time.Millisecond)
	_, ok := a.GetAnalysis("evt-low")
	if ok {
		t.Error("low severity events should not be analyzed")
	}
}

func TestHeuristicAnalysis_WithDependencyChain(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode(&ServiceNode{Name: "frontend"})
	g.AddNode(&ServiceNode{Name: "api"})
	g.AddNode(&ServiceNode{Name: "db"})
	_ = g.AddEdge("frontend", "api")
	_ = g.AddEdge("api", "db")
	a := New(g, nil)

	rc, err := a.AnalyzeIncident(context.Background(), "frontend", []engine.TelemetryEvent{
		{Source: "frontend", Severity: engine.SeverityCritical},
	})
	if err != nil {
		t.Fatalf("AnalyzeIncident failed: %v", err)
	}
	// CausalChain uses isHealthy=true (production default), so chain is empty since all healthy.
	// The heuristic still generates evidence-based output.
	if rc.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if rc.Confidence == 0 {
		t.Error("expected non-zero confidence")
	}
	if len(rc.Evidence) == 0 {
		t.Error("expected evidence in root cause")
	}
}

func TestBuildChainLinks(t *testing.T) {
	links := buildChainLinks([]string{"a", "b", "c"})
	if len(links) != 3 {
		t.Fatalf("expected 3 links, got %d", len(links))
	}
	if links[0].Service != "a" {
		t.Errorf("expected first link service 'a', got %s", links[0].Service)
	}
}
