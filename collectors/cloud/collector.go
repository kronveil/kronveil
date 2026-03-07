package cloud

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// ProviderType identifies the cloud provider.
type ProviderType string

const (
	ProviderAWS   ProviderType = "aws"
	ProviderAzure ProviderType = "azure"
	ProviderGCP   ProviderType = "gcp"
)

// Config holds the cloud collector configuration.
type Config struct {
	Provider     ProviderType  `yaml:"provider" json:"provider"`
	Regions      []string      `yaml:"regions" json:"regions"`
	PollInterval time.Duration `yaml:"poll_interval" json:"poll_interval"`
	Services     []string      `yaml:"services" json:"services"`
}

// CloudProvider is the interface each cloud provider must implement.
type CloudProvider interface {
	Name() string
	CollectMetrics(ctx context.Context) ([]CloudMetric, error)
	ListResources(ctx context.Context) ([]CloudResource, error)
}

// CloudMetric represents a metric from a cloud provider.
type CloudMetric struct {
	Provider  string                 `json:"provider"`
	Region    string                 `json:"region"`
	Service   string                 `json:"service"`
	Metric    string                 `json:"metric"`
	Value     float64                `json:"value"`
	Unit      string                 `json:"unit"`
	Dimensions map[string]string     `json:"dimensions"`
	Timestamp time.Time              `json:"timestamp"`
}

// CloudResource represents a tracked cloud resource.
type CloudResource struct {
	Provider   string            `json:"provider"`
	Region     string            `json:"region"`
	Service    string            `json:"service"`
	ResourceID string            `json:"resource_id"`
	Type       string            `json:"type"`
	Tags       map[string]string `json:"tags"`
	CostPerHour float64          `json:"cost_per_hour,omitempty"`
}

// Collector gathers telemetry from cloud providers.
type Collector struct {
	config   Config
	provider CloudProvider
	events   chan *engine.TelemetryEvent
	mu       sync.RWMutex
	running  bool
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	lastErr  error
}

// New creates a new cloud collector.
func New(config Config) (*Collector, error) {
	var provider CloudProvider
	switch config.Provider {
	case ProviderAWS:
		provider = NewAWSProvider(config.Regions)
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s", config.Provider)
	}

	return &Collector{
		config:   config,
		provider: provider,
		events:   make(chan *engine.TelemetryEvent, 500),
	}, nil
}

func (c *Collector) Name() string { return "cloud-" + string(c.config.Provider) }

func (c *Collector) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("cloud collector already running")
	}
	c.running = true
	ctx, c.cancel = context.WithCancel(ctx)
	c.mu.Unlock()

	log.Printf("[cloud-collector] Starting %s collector (regions: %v)", c.config.Provider, c.config.Regions)

	c.wg.Add(1)
	go c.pollMetrics(ctx)

	return nil
}

func (c *Collector) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return nil
	}
	c.running = false
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	close(c.events)
	return nil
}

func (c *Collector) Events() <-chan *engine.TelemetryEvent { return c.events }

func (c *Collector) Health() engine.ComponentHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()
	status := "healthy"
	msg := fmt.Sprintf("%s collector running across %d regions", c.config.Provider, len(c.config.Regions))
	if c.lastErr != nil {
		status = "degraded"
		msg = c.lastErr.Error()
	}
	return engine.ComponentHealth{
		Name:      fmt.Sprintf("cloud-%s-collector", c.config.Provider),
		Status:    status,
		Message:   msg,
		LastCheck: time.Now(),
	}
}

func (c *Collector) pollMetrics(ctx context.Context) {
	defer c.wg.Done()
	interval := c.config.PollInterval
	if interval == 0 {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics, err := c.provider.CollectMetrics(ctx)
			if err != nil {
				c.mu.Lock()
				c.lastErr = err
				c.mu.Unlock()
				log.Printf("[cloud-collector] Error collecting metrics: %v", err)
				continue
			}
			for _, m := range metrics {
				c.emitMetricEvent(m)
			}
		}
	}
}

func (c *Collector) emitMetricEvent(m CloudMetric) {
	event := &engine.TelemetryEvent{
		ID:        fmt.Sprintf("cloud-%d", time.Now().UnixNano()),
		Source:    "cloud",
		Type:      "cloud_metric",
		Timestamp: m.Timestamp,
		Payload: map[string]interface{}{
			"provider":   m.Provider,
			"region":     m.Region,
			"service":    m.Service,
			"metric":     m.Metric,
			"value":      m.Value,
			"unit":       m.Unit,
			"dimensions": m.Dimensions,
		},
		Metadata: map[string]string{
			"collector": "cloud",
			"provider":  m.Provider,
			"region":    m.Region,
		},
		Severity: engine.SeverityInfo,
	}
	select {
	case c.events <- event:
	default:
	}
}
