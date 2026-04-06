// Package kafka collects consumer lag and partition health metrics from Kafka clusters.
package kafka

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
	kafkago "github.com/segmentio/kafka-go"
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
	config      Config
	kafkaClient *kafkago.Client
	events      chan *engine.TelemetryEvent
	lagMon      *LagMonitor
	prevOffsets map[string]map[int]int64 // topic -> partition -> offset
	mu          sync.RWMutex
	running     bool
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	lastErr     error
	stats       struct {
		topicsMonitored int
		groupsMonitored int
		eventsEmitted   int64
	}
}

// New creates a new Kafka collector.
func New(config Config) *Collector {
	c := &Collector{
		config:      config,
		events:      make(chan *engine.TelemetryEvent, 500),
		lagMon:      NewLagMonitor(config.LagThreshold),
		prevOffsets: make(map[string]map[int]int64),
	}

	if config.BootstrapServers != "" {
		c.kafkaClient = &kafkago.Client{
			Addr: kafkago.TCP(config.BootstrapServers),
		}
	}

	return c
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
	if c.kafkaClient == nil {
		status = "degraded"
		msg = "no kafka client configured"
	} else if c.lastErr != nil {
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
				totalLag := c.fetchConsumerLag(ctx, group)

				severity := engine.SeverityInfo
				if totalLag > c.config.LagThreshold {
					severity = engine.SeverityHigh
				}
				c.emitEvent("consumer_lag", map[string]interface{}{
					"consumer_group": group,
					"total_lag":      totalLag,
					"threshold":      c.config.LagThreshold,
				}, severity)
			}
			c.mu.Lock()
			c.stats.groupsMonitored = len(c.config.ConsumerGroups)
			c.mu.Unlock()
		}
	}
}

func (c *Collector) fetchConsumerLag(ctx context.Context, group string) int64 {
	if c.kafkaClient == nil {
		return c.lagMon.GetGroupLag(group)
	}

	// Fetch committed offsets for the group.
	offsetResp, err := c.kafkaClient.OffsetFetch(ctx, &kafkago.OffsetFetchRequest{
		GroupID: group,
	})
	if err != nil {
		log.Printf("[kafka-collector] Failed to fetch offsets for group %s: %v", group, err)
		return c.lagMon.GetGroupLag(group)
	}

	var totalLag int64
	for topic, partitions := range offsetResp.Topics {
		for _, p := range partitions {
			// Fetch the latest offset for this partition using a direct connection.
			conn, err := kafkago.DialLeader(ctx, "tcp", c.config.BootstrapServers, topic, p.Partition)
			if err != nil {
				log.Printf("[kafka-collector] Failed to dial leader for %s/%d: %v", topic, p.Partition, err)
				continue
			}
			endOffset, err := conn.ReadLastOffset()
			_ = conn.Close()
			if err != nil {
				continue
			}

			lag := endOffset - p.CommittedOffset
			if lag > 0 {
				totalLag += lag
				c.lagMon.UpdateLag(group, int32(p.Partition), p.CommittedOffset, endOffset)
			}
		}
	}

	return totalLag
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
			if c.kafkaClient == nil {
				for _, topic := range c.config.MonitoredTopics {
					c.emitEvent("partition_health", map[string]interface{}{
						"topic": topic,
						"type":  "partition_health_check",
					}, engine.SeverityInfo)
				}
				continue
			}

			// Fetch metadata to check ISR counts and under-replicated partitions.
			conn, err := kafkago.Dial("tcp", c.config.BootstrapServers)
			if err != nil {
				c.mu.Lock()
				c.lastErr = err
				c.mu.Unlock()
				log.Printf("[kafka-collector] Failed to connect to Kafka: %v", err)
				continue
			}

			for _, topic := range c.config.MonitoredTopics {
				partitions, err := conn.ReadPartitions(topic)
				if err != nil {
					log.Printf("[kafka-collector] Failed to read partitions for %s: %v", topic, err)
					continue
				}

				c.mu.Lock()
				c.stats.topicsMonitored = len(c.config.MonitoredTopics)
				c.mu.Unlock()

				for _, p := range partitions {
					underReplicated := len(p.Replicas) - len(p.Isr)
					severity := engine.SeverityInfo
					if underReplicated > 0 {
						severity = engine.SeverityHigh
					}
					c.emitEvent("partition_health", map[string]interface{}{
						"topic":            topic,
						"partition":        p.ID,
						"leader":           p.Leader.ID,
						"replicas":         len(p.Replicas),
						"isr":              len(p.Isr),
						"under_replicated": underReplicated,
					}, severity)
				}
			}
			_ = conn.Close()
		}
	}
}

func (c *Collector) monitorThroughput(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(c.config.PollInterval)
	defer ticker.Stop()

	lastPoll := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.kafkaClient == nil {
				for _, topic := range c.config.MonitoredTopics {
					c.emitEvent("throughput", map[string]interface{}{
						"topic": topic,
						"type":  "cluster_throughput",
					}, engine.SeverityInfo)
				}
				continue
			}

			conn, err := kafkago.Dial("tcp", c.config.BootstrapServers)
			if err != nil {
				c.mu.Lock()
				c.lastErr = err
				c.mu.Unlock()
				log.Printf("[kafka-collector] Failed to connect to Kafka for throughput: %v", err)
				continue
			}

			now := time.Now()
			elapsed := now.Sub(lastPoll).Seconds()
			if elapsed <= 0 {
				elapsed = 1
			}
			lastPoll = now

			var totalClusterThroughput float64

			for _, topic := range c.config.MonitoredTopics {
				partitions, err := conn.ReadPartitions(topic)
				if err != nil {
					log.Printf("[kafka-collector] Failed to read partitions for %s: %v", topic, err)
					continue
				}

				var topicThroughput float64

				for _, p := range partitions {
					leaderAddr := fmt.Sprintf("%s:%d", p.Leader.Host, p.Leader.Port)
					pConn, err := kafkago.Dial("tcp", leaderAddr)
					if err != nil {
						log.Printf("[kafka-collector] Failed to dial leader %s for %s/%d: %v",
							leaderAddr, topic, p.ID, err)
						continue
					}

					offset, err := pConn.ReadLastOffset()
					_ = pConn.Close()
					if err != nil {
						log.Printf("[kafka-collector] Failed to read last offset for %s/%d: %v",
							topic, p.ID, err)
						continue
					}

					if prev, ok := c.prevOffsets[topic]; ok {
						if prevOffset, ok := prev[p.ID]; ok {
							diff := offset - prevOffset
							if diff > 0 {
								rate := float64(diff) / elapsed
								topicThroughput += rate
							}
						}
					}

					// Store current offset for next poll.
					if c.prevOffsets[topic] == nil {
						c.prevOffsets[topic] = make(map[int]int64)
					}
					c.prevOffsets[topic][p.ID] = offset
				}

				totalClusterThroughput += topicThroughput

				c.emitEvent("throughput", map[string]interface{}{
					"topic":        topic,
					"messages_sec": topicThroughput,
					"type":         "topic_throughput",
				}, engine.SeverityInfo)
			}

			_ = conn.Close()

			c.emitEvent("throughput", map[string]interface{}{
				"messages_sec": totalClusterThroughput,
				"type":         "cluster_throughput",
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
