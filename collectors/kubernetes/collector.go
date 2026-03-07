package kubernetes

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// Config holds the Kubernetes collector configuration.
type Config struct {
	Kubeconfig       string        `yaml:"kubeconfig" json:"kubeconfig"`
	Namespaces       []string      `yaml:"namespaces" json:"namespaces"`
	PollInterval     time.Duration `yaml:"poll_interval" json:"poll_interval"`
	MetricsInterval  time.Duration `yaml:"metrics_interval" json:"metrics_interval"`
	WatchEvents      bool          `yaml:"watch_events" json:"watch_events"`
	CollectMetrics   bool          `yaml:"collect_metrics" json:"collect_metrics"`
}

// DefaultConfig returns the default Kubernetes collector configuration.
func DefaultConfig() Config {
	return Config{
		PollInterval:    30 * time.Second,
		MetricsInterval: 15 * time.Second,
		WatchEvents:     true,
		CollectMetrics:  true,
	}
}

// Collector gathers telemetry from Kubernetes clusters.
type Collector struct {
	config   Config
	events   chan *engine.TelemetryEvent
	mu       sync.RWMutex
	running  bool
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	lastErr  error
	stats    collectorStats
}

type collectorStats struct {
	podsWatched    int
	nodesWatched   int
	eventsEmitted  int64
	errorsCount    int64
	lastEventTime  time.Time
}

// New creates a new Kubernetes collector.
func New(config Config) *Collector {
	return &Collector{
		config: config,
		events: make(chan *engine.TelemetryEvent, 1000),
	}
}

// Name returns the collector name.
func (c *Collector) Name() string { return "kubernetes" }

// Start begins collecting Kubernetes telemetry.
func (c *Collector) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("kubernetes collector already running")
	}
	c.running = true
	ctx, c.cancel = context.WithCancel(ctx)
	c.mu.Unlock()

	log.Printf("[k8s-collector] Starting Kubernetes collector (namespaces: %v, poll: %s)",
		c.config.Namespaces, c.config.PollInterval)

	// Watch for pod lifecycle events.
	c.wg.Add(1)
	go c.watchPods(ctx)

	// Watch Kubernetes events (warnings, errors).
	if c.config.WatchEvents {
		c.wg.Add(1)
		go c.watchEvents(ctx)
	}

	// Collect node and pod metrics.
	if c.config.CollectMetrics {
		c.wg.Add(1)
		go c.collectMetrics(ctx)
	}

	log.Println("[k8s-collector] Kubernetes collector started")
	return nil
}

// Stop halts the Kubernetes collector.
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
	log.Println("[k8s-collector] Kubernetes collector stopped")
	return nil
}

// Events returns the channel of telemetry events.
func (c *Collector) Events() <-chan *engine.TelemetryEvent { return c.events }

// Health returns the collector health status.
func (c *Collector) Health() engine.ComponentHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()
	status := "healthy"
	msg := fmt.Sprintf("watching %d pods, %d nodes", c.stats.podsWatched, c.stats.nodesWatched)
	if c.lastErr != nil {
		status = "degraded"
		msg = c.lastErr.Error()
	}
	return engine.ComponentHealth{
		Name:      "kubernetes-collector",
		Status:    status,
		Message:   msg,
		LastCheck: time.Now(),
	}
}

func (c *Collector) watchPods(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(c.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.emitEvent("pod_status", map[string]interface{}{
				"type":       "pod_health_check",
				"pods_total": c.stats.podsWatched,
			}, engine.SeverityInfo)
		}
	}
}

func (c *Collector) watchEvents(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// In production: uses client-go Watch on core/v1 Events.
			// Emits events for OOMKilled, CrashLoopBackOff, FailedScheduling, etc.
		}
	}
}

func (c *Collector) collectMetrics(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(c.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// In production: queries metrics-server for node/pod resource usage.
			c.emitEvent("node_metrics", map[string]interface{}{
				"type":        "node_resource_usage",
				"nodes_total": c.stats.nodesWatched,
			}, engine.SeverityInfo)
		}
	}
}

func (c *Collector) emitEvent(eventType string, payload map[string]interface{}, severity string) {
	event := &engine.TelemetryEvent{
		ID:        fmt.Sprintf("k8s-%d", time.Now().UnixNano()),
		Source:    "kubernetes",
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   payload,
		Metadata:  map[string]string{"collector": "kubernetes"},
		Severity:  severity,
	}
	select {
	case c.events <- event:
		c.mu.Lock()
		c.stats.eventsEmitted++
		c.stats.lastEventTime = time.Now()
		c.mu.Unlock()
	default:
		log.Println("[k8s-collector] Event channel full, dropping event")
	}
}
