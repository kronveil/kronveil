package capacity

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

func TestLinearRegression(t *testing.T) {
	// y = 2x + 1: values at x=0,1,2,3,4 → 1,3,5,7,9
	values := []float64{1, 3, 5, 7, 9}
	slope, intercept := linearRegression(values)

	if math.Abs(slope-2.0) > 0.01 {
		t.Errorf("slope = %f, want 2.0", slope)
	}
	if math.Abs(intercept-1.0) > 0.01 {
		t.Errorf("intercept = %f, want 1.0", intercept)
	}
}

func TestLinearRegression_SingleValue(t *testing.T) {
	slope, intercept := linearRegression([]float64{5})
	if slope != 0 || intercept != 0 {
		t.Errorf("single value: slope=%f intercept=%f, want 0,0", slope, intercept)
	}
}

func TestGenerateForecast_InsufficientData(t *testing.T) {
	p := New(Config{})

	// Record fewer than 10 points.
	for i := 0; i < 5; i++ {
		p.recordMetric("test-resource", "cpu", float64(i))
	}

	_, err := p.GenerateForecast("test-resource")
	if err == nil {
		t.Error("expected error for insufficient data")
	}
}

func TestGenerateForecast_Normal(t *testing.T) {
	p := New(Config{})

	// Record enough points.
	for i := 0; i < 20; i++ {
		p.recordMetric("test-resource", "cpu", float64(i*2+10))
	}

	forecast, err := p.GenerateForecast("test-resource")
	if err != nil {
		t.Fatalf("GenerateForecast failed: %v", err)
	}
	if forecast.Resource != "test-resource" {
		t.Errorf("unexpected resource: %s", forecast.Resource)
	}
	if len(forecast.ForecastValues) == 0 {
		t.Error("expected forecast values")
	}
	if forecast.Trend != "increasing" {
		t.Errorf("expected increasing trend, got %s", forecast.Trend)
	}
}

func TestGenerateForecast_NotFound(t *testing.T) {
	p := New(Config{})
	_, err := p.GenerateForecast("nonexistent")
	if err == nil {
		t.Error("expected error for unknown resource")
	}
}

func TestRecommendations_OverProvisioned(t *testing.T) {
	p := New(Config{})

	// Record values with low utilization (all < 30%).
	for i := 0; i < 20; i++ {
		p.recordMetric("low-usage", "cpu", 10+float64(i%5))
	}

	recs := p.GenerateRecommendations()
	found := false
	for _, r := range recs {
		if r.Resource == "low-usage" && r.Type == "scale_down" {
			found = true
		}
	}
	if !found {
		t.Error("expected scale_down recommendation for over-provisioned resource")
	}
}

func TestRecommendations_UnderProvisioned(t *testing.T) {
	p := New(Config{})

	// Record values with high utilization (all > 80%).
	for i := 0; i < 20; i++ {
		p.recordMetric("high-usage", "cpu", 85+float64(i%5))
	}

	recs := p.GenerateRecommendations()
	found := false
	for _, r := range recs {
		if r.Resource == "high-usage" && r.Type == "scale_up" {
			found = true
		}
	}
	if !found {
		t.Error("expected scale_up recommendation for under-provisioned resource")
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		input interface{}
		want  float64
		ok    bool
	}{
		{float64(3.14), 3.14, true},
		{float32(2.5), 2.5, true},
		{int(42), 42, true},
		{int64(100), 100, true},
		{"string", 0, false},
		{nil, 0, false},
	}

	for _, tt := range tests {
		got, ok := toFloat(tt.input)
		if ok != tt.ok {
			t.Errorf("toFloat(%v): ok=%v, want %v", tt.input, ok, tt.ok)
		}
		if ok && math.Abs(got-tt.want) > 0.01 {
			t.Errorf("toFloat(%v) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestAnalyze_RecordsMetric(t *testing.T) {
	p := New(Config{})
	_ = p.Start(context.Background())
	defer func() { _ = p.Stop() }()

	event := &engine.TelemetryEvent{
		Source:  "k8s",
		Type:    "metrics",
		Payload: map[string]interface{}{"cpu": 45.0, "memory": 60.0},
	}
	_ = p.Analyze(context.Background(), event)

	// Should have recorded both metrics.
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.resourceData) != 2 {
		t.Errorf("expected 2 resource entries, got %d", len(p.resourceData))
	}
}

func TestDataRetention_Trimming(t *testing.T) {
	p := New(Config{
		DataRetention: 1 * time.Millisecond,
	})

	p.recordMetric("test", "cpu", 50)
	time.Sleep(5 * time.Millisecond)
	p.recordMetric("test", "cpu", 60)

	p.mu.RLock()
	ts := p.resourceData["test"]
	p.mu.RUnlock()

	// Old point should be trimmed.
	if len(ts.Points) > 1 {
		t.Errorf("expected old data trimmed, got %d points", len(ts.Points))
	}
}

func TestPlanner_Name(t *testing.T) {
	p := New(Config{})
	if p.Name() != "capacity-planner" {
		t.Errorf("expected 'capacity-planner', got %s", p.Name())
	}
}

func TestPlanner_Health(t *testing.T) {
	p := New(Config{})
	h := p.Health()
	if h.Name != "capacity-planner" {
		t.Errorf("expected name 'capacity-planner', got %s", h.Name)
	}
	if h.Status != "healthy" {
		t.Errorf("expected healthy, got %s", h.Status)
	}
}

func TestPlanner_StartStop(t *testing.T) {
	p := New(Config{})
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestStandardError(t *testing.T) {
	// Less than 3 values.
	se := standardError([]float64{1, 2}, 1, 0)
	if se != 0 {
		t.Errorf("standardError with <3 values should be 0, got %f", se)
	}

	// Perfect fit → 0 error.
	se = standardError([]float64{0, 1, 2, 3}, 1, 0)
	if se != 0 {
		t.Errorf("standardError for perfect fit should be 0, got %f", se)
	}
}

func TestPercentile95(t *testing.T) {
	if percentile95(nil) != 0 {
		t.Error("percentile95 of empty should be 0")
	}
	vals := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	p := percentile95(vals)
	if p == 0 {
		t.Error("expected non-zero p95")
	}
}

func TestMean_Capacity(t *testing.T) {
	if mean(nil) != 0 {
		t.Error("mean of empty should be 0")
	}
	m := mean([]float64{10, 20, 30})
	if m != 20 {
		t.Errorf("mean = %f, want 20", m)
	}
}

func TestGenerateForecast_StableTrend(t *testing.T) {
	p := New(Config{})
	// Record constant values → stable trend.
	for i := 0; i < 20; i++ {
		p.recordMetric("stable-resource", "cpu", 50.0)
	}

	forecast, err := p.GenerateForecast("stable-resource")
	if err != nil {
		t.Fatalf("GenerateForecast failed: %v", err)
	}
	if forecast.Trend != "stable" {
		t.Errorf("expected stable trend, got %s", forecast.Trend)
	}
}

func TestGenerateForecast_DecreasingTrend(t *testing.T) {
	p := New(Config{})
	for i := 0; i < 20; i++ {
		p.recordMetric("decreasing", "cpu", 100.0-float64(i)*5)
	}

	forecast, err := p.GenerateForecast("decreasing")
	if err != nil {
		t.Fatalf("GenerateForecast failed: %v", err)
	}
	if forecast.Trend != "decreasing" {
		t.Errorf("expected decreasing trend, got %s", forecast.Trend)
	}
}

func TestRecommendations_InsufficientData(t *testing.T) {
	p := New(Config{})
	// Only 5 data points, not enough.
	for i := 0; i < 5; i++ {
		p.recordMetric("short-data", "cpu", 10.0)
	}

	recs := p.GenerateRecommendations()
	for _, r := range recs {
		if r.Resource == "short-data" {
			t.Error("should not generate recommendations with insufficient data")
		}
	}
}
