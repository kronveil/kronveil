// Package capacity provides resource capacity forecasting and right-sizing.
package capacity

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// Config holds capacity planner configuration.
type Config struct {
	ForecastHorizon time.Duration `yaml:"forecast_horizon" json:"forecast_horizon"`
	DataRetention   time.Duration `yaml:"data_retention" json:"data_retention"`
	SampleInterval  time.Duration `yaml:"sample_interval" json:"sample_interval"`
}

// Recommendation represents a capacity right-sizing recommendation.
type Recommendation struct {
	Resource      string    `json:"resource"`
	Type          string    `json:"type"` // "scale_up", "scale_down", "right_size", "optimize"
	Current       float64   `json:"current"`
	Recommended   float64   `json:"recommended"`
	SavingsPercent float64  `json:"savings_percent,omitempty"`
	Reason        string    `json:"reason"`
	Confidence    float64   `json:"confidence"`
	Timestamp     time.Time `json:"timestamp"`
}

// Forecast represents a capacity forecast.
type Forecast struct {
	Resource       string       `json:"resource"`
	Metric         string       `json:"metric"`
	CurrentValue   float64      `json:"current_value"`
	ForecastValues []ForecastPoint `json:"forecast_values"`
	Trend          string       `json:"trend"` // "increasing", "stable", "decreasing"
	DaysToCapacity *int         `json:"days_to_capacity,omitempty"`
}

// ForecastPoint is a single forecasted data point.
type ForecastPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	Value      float64   `json:"value"`
	LowerBound float64   `json:"lower_bound"`
	UpperBound float64   `json:"upper_bound"`
}

// Planner performs capacity planning and right-sizing analysis.
type Planner struct {
	config          Config
	mu              sync.RWMutex
	resourceData map[string]*ResourceTimeSeries
	running      bool
	cancel          context.CancelFunc
}

// ResourceTimeSeries stores historical resource utilization data.
type ResourceTimeSeries struct {
	Resource string
	Metric   string
	Points   []DataPoint
}

// DataPoint is a timestamped metric value.
type DataPoint struct {
	Timestamp time.Time
	Value     float64
}

// New creates a new capacity planner.
func New(config Config) *Planner {
	if config.ForecastHorizon == 0 {
		config.ForecastHorizon = 30 * 24 * time.Hour // 30 days
	}
	if config.DataRetention == 0 {
		config.DataRetention = 90 * 24 * time.Hour // 90 days
	}
	if config.SampleInterval == 0 {
		config.SampleInterval = 5 * time.Minute
	}

	return &Planner{
		config:       config,
		resourceData: make(map[string]*ResourceTimeSeries),
	}
}

func (p *Planner) Name() string { return "capacity-planner" }

func (p *Planner) Start(ctx context.Context) error {
	p.mu.Lock()
	p.running = true
	_, p.cancel = context.WithCancel(ctx)
	p.mu.Unlock()
	log.Printf("[capacity] Capacity planner started (forecast: %s, retention: %s)",
		p.config.ForecastHorizon, p.config.DataRetention)
	return nil
}

func (p *Planner) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.running = false
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

func (p *Planner) Analyze(ctx context.Context, event *engine.TelemetryEvent) error {
	// Record resource utilization metrics for capacity planning.
	for key, val := range event.Payload {
		if v, ok := toFloat(val); ok {
			resourceKey := fmt.Sprintf("%s.%s.%s", event.Source, event.Type, key)
			p.recordMetric(resourceKey, key, v)
		}
	}
	return nil
}

func (p *Planner) Health() engine.ComponentHealth {
	return engine.ComponentHealth{
		Name:      "capacity-planner",
		Status:    "healthy",
		Message:   fmt.Sprintf("tracking %d resources", len(p.resourceData)),
		LastCheck: time.Now(),
	}
}

func (p *Planner) recordMetric(resource, metric string, value float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ts, ok := p.resourceData[resource]
	if !ok {
		ts = &ResourceTimeSeries{Resource: resource, Metric: metric}
		p.resourceData[resource] = ts
	}

	ts.Points = append(ts.Points, DataPoint{
		Timestamp: time.Now(),
		Value:     value,
	})

	// Trim old data points.
	cutoff := time.Now().Add(-p.config.DataRetention)
	firstValid := 0
	for i, pt := range ts.Points {
		if pt.Timestamp.After(cutoff) {
			firstValid = i
			break
		}
	}
	if firstValid > 0 {
		ts.Points = ts.Points[firstValid:]
	}
}

// GenerateForecast creates a capacity forecast for a resource.
func (p *Planner) GenerateForecast(resource string) (*Forecast, error) {
	p.mu.RLock()
	ts, ok := p.resourceData[resource]
	p.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no data for resource %s", resource)
	}

	if len(ts.Points) < 10 {
		return nil, fmt.Errorf("insufficient data points for forecast (%d < 10)", len(ts.Points))
	}

	values := make([]float64, len(ts.Points))
	for i, pt := range ts.Points {
		values[i] = pt.Value
	}

	// Linear regression for trend.
	slope, intercept := linearRegression(values)

	// Generate forecast points.
	forecastPoints := make([]ForecastPoint, 0)
	now := time.Now()
	n := len(values)
	stdErr := standardError(values, slope, intercept)

	steps := 30 // 30 forecast periods
	for i := 1; i <= steps; i++ {
		x := float64(n + i)
		predicted := slope*x + intercept
		t := now.Add(time.Duration(i) * 24 * time.Hour)

		forecastPoints = append(forecastPoints, ForecastPoint{
			Timestamp:  t,
			Value:      predicted,
			LowerBound: predicted - 1.96*stdErr,
			UpperBound: predicted + 1.96*stdErr,
		})
	}

	// Determine trend.
	trend := "stable"
	if slope > 0.01 {
		trend = "increasing"
	} else if slope < -0.01 {
		trend = "decreasing"
	}

	forecast := &Forecast{
		Resource:       resource,
		Metric:         ts.Metric,
		CurrentValue:   values[len(values)-1],
		ForecastValues: forecastPoints,
		Trend:          trend,
	}

	// Calculate days to capacity (assuming 100% is capacity).
	if slope > 0 && values[len(values)-1] < 100 {
		daysToCapacity := int((100 - values[len(values)-1]) / (slope * 24))
		if daysToCapacity > 0 && daysToCapacity < 365 {
			forecast.DaysToCapacity = &daysToCapacity
		}
	}

	return forecast, nil
}

// GenerateRecommendations produces right-sizing recommendations.
func (p *Planner) GenerateRecommendations() []Recommendation {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var recommendations []Recommendation

	for resource, ts := range p.resourceData {
		if len(ts.Points) < 10 {
			continue
		}

		values := make([]float64, len(ts.Points))
		for i, pt := range ts.Points {
			values[i] = pt.Value
		}

		avg := mean(values)
		p95 := percentile95(values)

		// Over-provisioned: p95 usage < 30%.
		if p95 < 30 {
			recommendations = append(recommendations, Recommendation{
				Resource:       resource,
				Type:           "scale_down",
				Current:        100,
				Recommended:    math.Ceil(p95 * 1.5),
				SavingsPercent: (100 - math.Ceil(p95*1.5)),
				Reason:         fmt.Sprintf("P95 utilization is only %.1f%%, resource is over-provisioned", p95),
				Confidence:     0.8,
				Timestamp:      time.Now(),
			})
		}

		// Under-provisioned: p95 usage > 80%.
		if p95 > 80 {
			recommendations = append(recommendations, Recommendation{
				Resource:    resource,
				Type:        "scale_up",
				Current:     100,
				Recommended: math.Ceil(avg * 2),
				Reason:      fmt.Sprintf("P95 utilization is %.1f%%, at risk of saturation", p95),
				Confidence:  0.85,
				Timestamp:   time.Now(),
			})
		}
	}

	return recommendations
}

func linearRegression(values []float64) (slope, intercept float64) {
	n := float64(len(values))
	if n < 2 {
		return 0, 0
	}
	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, sumY / n
	}
	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n
	return
}

func standardError(values []float64, slope, intercept float64) float64 {
	n := float64(len(values))
	if n < 3 {
		return 0
	}
	var sumSqErr float64
	for i, y := range values {
		predicted := slope*float64(i) + intercept
		err := y - predicted
		sumSqErr += err * err
	}
	return math.Sqrt(sumSqErr / (n - 2))
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func percentile95(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	// Simple selection for p95.
	idx := int(0.95 * float64(len(sorted)-1))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}
