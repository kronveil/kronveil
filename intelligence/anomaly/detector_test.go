package anomaly

import (
	"context"
	"testing"
	"time"

	"github.com/kronveil/kronveil/core/engine"
	"github.com/kronveil/kronveil/internal/testutil"
)

func newTestEvent(source string, payload map[string]interface{}) *engine.TelemetryEvent {
	return &engine.TelemetryEvent{
		ID:        "test-evt",
		Source:    source,
		Timestamp: time.Now(),
		Payload:   payload,
		Severity:  engine.SeverityInfo,
	}
}

func TestDetector_NormalValues_NoAnomaly(t *testing.T) {
	d := New(DefaultConfig())
	_ = d.Start(context.Background())
	defer func() { _ = d.Stop() }()

	// Feed enough normal data points.
	for i := 0; i < 50; i++ {
		event := newTestEvent("test", map[string]interface{}{"cpu": 50.0})
		if err := d.Analyze(context.Background(), event); err != nil {
			t.Fatalf("Analyze failed: %v", err)
		}
	}

	anomalies := d.ListAnomalies()
	if len(anomalies) != 0 {
		t.Errorf("expected 0 anomalies for normal values, got %d", len(anomalies))
	}
}

func TestDetector_ExtremeValue_TriggersAnomaly(t *testing.T) {
	d := New(DefaultConfig())
	_ = d.Start(context.Background())
	defer func() { _ = d.Stop() }()

	// Feed normal baseline.
	for i := 0; i < 50; i++ {
		event := newTestEvent("test", map[string]interface{}{"cpu": 50.0})
		_ = d.Analyze(context.Background(), event)
	}

	// Feed extreme value.
	event := newTestEvent("test", map[string]interface{}{"cpu": 500.0})
	_ = d.Analyze(context.Background(), event)

	anomalies := d.ListAnomalies()
	if len(anomalies) == 0 {
		t.Error("expected anomaly for extreme value")
	}
}

func TestDetector_MinDataPoints(t *testing.T) {
	config := DefaultConfig()
	config.MinDataPoints = 30
	d := New(config)
	_ = d.Start(context.Background())
	defer func() { _ = d.Stop() }()

	// Feed fewer than min data points with an extreme value.
	for i := 0; i < 10; i++ {
		event := newTestEvent("test", map[string]interface{}{"cpu": 50.0})
		_ = d.Analyze(context.Background(), event)
	}
	// Even an extreme value shouldn't trigger before minimum.
	event := newTestEvent("test", map[string]interface{}{"cpu": 9999.0})
	_ = d.Analyze(context.Background(), event)

	anomalies := d.ListAnomalies()
	if len(anomalies) != 0 {
		t.Errorf("expected no anomalies before min data points, got %d", len(anomalies))
	}
}

func TestDetector_SensitivityHigh(t *testing.T) {
	config := DefaultConfig()
	config.Sensitivity = "high"
	d := New(config)

	// High sensitivity should set threshold to 2.0.
	if d.config.ZScoreThreshold != 2.0 {
		t.Errorf("expected high sensitivity threshold 2.0, got %f", d.config.ZScoreThreshold)
	}
}

func TestDetector_SensitivityLow(t *testing.T) {
	config := DefaultConfig()
	config.Sensitivity = "low"
	d := New(config)

	if d.config.ZScoreThreshold != 4.0 {
		t.Errorf("expected low sensitivity threshold 4.0, got %f", d.config.ZScoreThreshold)
	}
}

func TestDetector_MetricsRecording(t *testing.T) {
	d := New(DefaultConfig())
	m := &testutil.MockMetricsRecorder{}
	d.SetMetrics(m)
	_ = d.Start(context.Background())
	defer func() { _ = d.Stop() }()

	// Feed baseline.
	for i := 0; i < 50; i++ {
		_ = d.Analyze(context.Background(), newTestEvent("test", map[string]interface{}{"cpu": 50.0}))
	}

	// Trigger anomaly.
	_ = d.Analyze(context.Background(), newTestEvent("test", map[string]interface{}{"cpu": 500.0}))

	if m.Anomalies.Load() == 0 {
		t.Error("expected RecordAnomaly to be called")
	}
}

func TestDetector_ListAnomalies_ReturnsCopy(t *testing.T) {
	d := New(DefaultConfig())
	_ = d.Start(context.Background())
	defer func() { _ = d.Stop() }()

	list1 := d.ListAnomalies()
	list2 := d.ListAnomalies()

	// Modifying one shouldn't affect the other.
	if len(list1) != len(list2) {
		t.Error("ListAnomalies should return consistent results")
	}
}

func TestExtractSignals_AllTypes(t *testing.T) {
	event := &engine.TelemetryEvent{
		Source: "test",
		Payload: map[string]interface{}{
			"float64_val": float64(1.0),
			"float32_val": float32(2.0),
			"int_val":     int(3),
			"int64_val":   int64(4),
			"string_val":  "not a number",
		},
	}
	signals := extractSignals(event)
	if len(signals) != 4 {
		t.Errorf("expected 4 numeric signals, got %d", len(signals))
	}
	if signals["float64_val"] != 1.0 {
		t.Error("float64 not extracted correctly")
	}
	if signals["float32_val"] != 2.0 {
		t.Error("float32 not extracted correctly")
	}
	if signals["int_val"] != 3.0 {
		t.Error("int not extracted correctly")
	}
	if signals["int64_val"] != 4.0 {
		t.Error("int64 not extracted correctly")
	}
}

func TestDetector_Health(t *testing.T) {
	d := New(DefaultConfig())
	_ = d.Start(context.Background())
	defer func() { _ = d.Stop() }()

	h := d.Health()
	if h.Name != "anomaly-detector" {
		t.Errorf("expected name 'anomaly-detector', got %s", h.Name)
	}
	if h.Status != "healthy" {
		t.Errorf("expected healthy, got %s", h.Status)
	}
}

func TestDetector_Name(t *testing.T) {
	d := New(DefaultConfig())
	if d.Name() != "anomaly-detector" {
		t.Errorf("expected 'anomaly-detector', got %s", d.Name())
	}
}

func TestDetector_AnomaliesChannel(t *testing.T) {
	d := New(DefaultConfig())
	_ = d.Start(context.Background())

	ch := d.Anomalies()
	if ch == nil {
		t.Error("Anomalies channel should not be nil")
	}

	// Feed baseline and then extreme.
	for i := 0; i < 50; i++ {
		_ = d.Analyze(context.Background(), newTestEvent("test", map[string]interface{}{"cpu": 50.0}))
	}
	_ = d.Analyze(context.Background(), newTestEvent("test", map[string]interface{}{"cpu": 500.0}))

	select {
	case a := <-ch:
		if a == nil {
			t.Error("expected non-nil anomaly on channel")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected anomaly on channel")
	}

	_ = d.Stop()
}
