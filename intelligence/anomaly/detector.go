package anomaly

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// Config holds anomaly detection configuration.
type Config struct {
	WindowSize       int     `yaml:"window_size" json:"window_size"`
	ZScoreThreshold  float64 `yaml:"zscore_threshold" json:"zscore_threshold"`
	Sensitivity      string  `yaml:"sensitivity" json:"sensitivity"` // low, medium, high
	EWMAAlpha        float64 `yaml:"ewma_alpha" json:"ewma_alpha"`
	MinDataPoints    int     `yaml:"min_data_points" json:"min_data_points"`
}

// DefaultConfig returns sensible defaults for anomaly detection.
func DefaultConfig() Config {
	return Config{
		WindowSize:      300,
		ZScoreThreshold: 3.0,
		Sensitivity:     "medium",
		EWMAAlpha:       0.1,
		MinDataPoints:   30,
	}
}

// Detector performs anomaly detection on telemetry signals.
type Detector struct {
	config   Config
	mu       sync.RWMutex
	signals  map[string]*SignalState
	anomalyCh chan *engine.Anomaly
	running  bool
	cancel   context.CancelFunc
}

// SignalState tracks the state of a monitored signal.
type SignalState struct {
	Name       string
	Window     *TimeSeriesWindow
	EWMA       *EWMA
	LastValue  float64
	LastUpdate time.Time
	AnomalyCount int
}

// New creates a new anomaly detector.
func New(config Config) *Detector {
	applyeSensitivity(&config)
	return &Detector{
		config:    config,
		signals:   make(map[string]*SignalState),
		anomalyCh: make(chan *engine.Anomaly, 100),
	}
}

func applyeSensitivity(config *Config) {
	switch config.Sensitivity {
	case "high":
		config.ZScoreThreshold = 2.0
	case "low":
		config.ZScoreThreshold = 4.0
	default: // medium
		config.ZScoreThreshold = 3.0
	}
}

// Name returns the module name.
func (d *Detector) Name() string { return "anomaly-detector" }

// Start begins the anomaly detection module.
func (d *Detector) Start(ctx context.Context) error {
	d.mu.Lock()
	d.running = true
	ctx, d.cancel = context.WithCancel(ctx)
	d.mu.Unlock()

	log.Printf("[anomaly] Anomaly detector started (window: %d, threshold: %.1f, sensitivity: %s)",
		d.config.WindowSize, d.config.ZScoreThreshold, d.config.Sensitivity)
	return nil
}

// Stop halts the anomaly detector.
func (d *Detector) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.running = false
	if d.cancel != nil {
		d.cancel()
	}
	close(d.anomalyCh)
	return nil
}

// Analyze processes a telemetry event for anomalies.
func (d *Detector) Analyze(ctx context.Context, event *engine.TelemetryEvent) error {
	// Extract numeric signals from the event payload.
	signals := extractSignals(event)

	for signalName, value := range signals {
		key := fmt.Sprintf("%s.%s", event.Source, signalName)
		anomaly := d.analyzeSignal(key, value, event)
		if anomaly != nil {
			select {
			case d.anomalyCh <- anomaly:
			default:
				log.Println("[anomaly] Anomaly channel full, dropping")
			}
		}
	}

	return nil
}

// Health returns the detector health status.
func (d *Detector) Health() engine.ComponentHealth {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return engine.ComponentHealth{
		Name:      "anomaly-detector",
		Status:    "healthy",
		Message:   fmt.Sprintf("tracking %d signals", len(d.signals)),
		LastCheck: time.Now(),
	}
}

// Anomalies returns the channel of detected anomalies.
func (d *Detector) Anomalies() <-chan *engine.Anomaly {
	return d.anomalyCh
}

func (d *Detector) analyzeSignal(key string, value float64, event *engine.TelemetryEvent) *engine.Anomaly {
	d.mu.Lock()
	state, ok := d.signals[key]
	if !ok {
		state = &SignalState{
			Name:   key,
			Window: NewTimeSeriesWindow(d.config.WindowSize),
			EWMA:   NewEWMA(d.config.EWMAAlpha),
		}
		d.signals[key] = state
	}
	d.mu.Unlock()

	state.Window.Add(value)
	state.EWMA.Add(value)
	state.LastValue = value
	state.LastUpdate = time.Now()

	// Need minimum data points before detecting anomalies.
	if state.Window.Len() < d.config.MinDataPoints {
		return nil
	}

	values := state.Window.Values()
	zscore := ZScore(value, values)
	absZScore := math.Abs(zscore)

	// Check for anomaly using z-score threshold.
	if absZScore < d.config.ZScoreThreshold {
		return nil
	}

	// Calculate anomaly score (0.0 to 1.0).
	score := math.Min(absZScore/6.0, 1.0)

	// Check if this is a predicted anomaly (trend-based).
	trend := LinearTrend(values)
	predicted := false
	if math.Abs(trend) > StdDev(values)*0.1 {
		predicted = true
	}

	// Determine severity based on score.
	severity := engine.SeverityLow
	switch {
	case score >= 0.9:
		severity = engine.SeverityCritical
	case score >= 0.7:
		severity = engine.SeverityHigh
	case score >= 0.5:
		severity = engine.SeverityMedium
	}

	state.AnomalyCount++

	return &engine.Anomaly{
		ID:          fmt.Sprintf("ano-%s-%d", key, time.Now().UnixNano()),
		Signal:      key,
		Score:       score,
		Timestamp:   time.Now(),
		Predicted:   predicted,
		Description: fmt.Sprintf("Signal %s value %.2f deviates %.1f sigma from mean (threshold: %.1f)", key, value, absZScore, d.config.ZScoreThreshold),
		Source:      event.Source,
		Severity:    severity,
		Threshold:   d.config.ZScoreThreshold,
	}
}

func extractSignals(event *engine.TelemetryEvent) map[string]float64 {
	signals := make(map[string]float64)
	for k, v := range event.Payload {
		switch val := v.(type) {
		case float64:
			signals[k] = val
		case float32:
			signals[k] = float64(val)
		case int:
			signals[k] = float64(val)
		case int64:
			signals[k] = float64(val)
		}
	}
	return signals
}
