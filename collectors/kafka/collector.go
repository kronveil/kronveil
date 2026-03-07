package kafka

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// Config holds the Kafka collector configuration.
type Config struct {
	BootstrapServers string        `yaml:"bootstrap_servers" json:"bootstrap_servers"`
	MonitoredTopics  []string      `yaml:"monitored_topics" json:"monitored_topics"`
	ConsumerGroups   []string      `yaml:"consumer_groups" json:"consumer_groups"`
	PollInterval     time.Duration `yaml:"poll_interval" json:"poll_interval"`
	LagThreshold     int64         `yaml:"lag_threshold" json:"lag_threshold"`
}

// DefaultConfig returns the default Kafka collector configuration.
func DefaultConfig() Config {
	return Config{
		PollInterval: 10 * time.Second,
		LagThreshold: 10000,
	}
}

// Collector gathers telemetry from Apache Kafka clusters.
type Collector struct {
	config  Config
	events  chan *engine.TelemetryEvent
	lagMon  *LagMonitor
	mu      sync.RWMutex
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	lastErr error
	stats   struct {
		topicsMonitored int
		groupsMonitored int
		eventsEmitted   int64
	}
}

// New creates a new Kafka collector.
func New(config Config) *Collector {
	return &Collector{
		config: config,
		events: make(chan *engine.TelemetryEvent, 500),
		lagMon: NewLagMonitor(config.LagThreshold),
	}
}

// Name returns the collector name.
func (c *Collector) Name() string { return "kafka" }

// Start begins collecting Kafka telemetry.
func (c *Collector) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("kafka collector already running")
	}
	c.running = true
	_, c.cancel = context.WithCancel(ctx)
	c.mu.Unlock()

	log.Printf("[kafka-collector] Starting Kafka collector (servers: %s, topics: %v)",
		c.config.BootstrapServers, c.config.MonitoredTopics)

	c.wg.Add(1)
	go c.monitorConsumerLag(ctx)

	c.wg.Add(1)
	go c.monitorPartitionHealth(ctx)

	c.wg.Add(1)
	go c.monitorThroughput(ctx)

	return nil
}

// Stop halts the Kafka collector.
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

// Events returns the channel of telemetry events.
func (c *Collector) Events() <-chan *engine.TelemetryEvent { return c.events }

// Health returns the collector health status.
func (c *Collector) Health() engine.ComponentHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()
	status := "healthy"
	msg := fmt.Sprintf("monitoring %d topics, %d consumer groups",
		c.stats.topicsMonitored, c.stats.groupsMonitored)
	if c.lastErr != nil {
		status = "degraded"
		msg = c.lastErr.Error()
	}
	return engine.ComponentHealth{
		Name:      "kafka-collector",
		Status:    status,
		Message:   msg,
		LastCheck: time.Now(),
	}
}

func (c *Collector) monitorConsumerLag(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(c.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, group := range c.config.ConsumerGroups {
				// In production: uses AdminClient to describe consumer groups and get offsets.
				lag := c.lagMon.GetGroupLag(group)
				severity := engine.SeverityInfo
				if lag > c.config.LagThreshold {
					severity = engine.SeverityHigh
				}
				c.emitEvent("consumer_lag", map[string]interface{}{
					"consumer_group": group,
					"total_lag":      lag,
					"threshold":      c.config.LagThreshold,
				}, severity)
			}
		}
	}
}

func (c *Collector) monitorPartitionHealth(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, topic := range c.config.MonitoredTopics {
				c.emitEvent("partition_health", map[string]interface{}{
					"topic": topic,
					"type":  "partition_health_check",
				}, engine.SeverityInfo)
			}
		}
	}
}

func (c *Collector) monitorThroughput(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(c.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.emitEvent("throughput", map[string]interface{}{
				"type": "cluster_throughput",
			}, engine.SeverityInfo)
		}
	}
}

func (c *Collector) emitEvent(eventType string, payload map[string]interface{}, severity string) {
	event := &engine.TelemetryEvent{
		ID:        fmt.Sprintf("kafka-%d", time.Now().UnixNano()),
		Source:    "kafka",
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   payload,
		Metadata:  map[string]string{"collector": "kafka"},
		Severity:  severity,
	}
	select {
	case c.events <- event:
		c.mu.Lock()
		c.stats.eventsEmitted++
		c.mu.Unlock()
	default:
		log.Println("[kafka-collector] Event channel full, dropping event")
	}
}
