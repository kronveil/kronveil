// Package eventbus provides the Kafka-backed event bus for Kronveil.
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// EventBus defines the interface for the event streaming backbone.
type EventBus interface {
	Publish(ctx context.Context, topic string, event *engine.TelemetryEvent) error
	Subscribe(ctx context.Context, topic string, group string, handler func(*engine.TelemetryEvent)) error
	Unsubscribe(topic string) error
	Close() error
	Metrics() BusMetrics
}

// BusMetrics tracks event bus throughput.
type BusMetrics struct {
	PublishedTotal  int64   `json:"published_total"`
	ConsumedTotal   int64   `json:"consumed_total"`
	PublishRate     float64 `json:"publish_rate_per_sec"`
	ConsumeRate     float64 `json:"consume_rate_per_sec"`
	ErrorsTotal     int64   `json:"errors_total"`
}

// KafkaConfig holds Kafka connection configuration.
type KafkaConfig struct {
	BootstrapServers string `yaml:"bootstrap_servers" json:"bootstrap_servers"`
	GroupID          string `yaml:"group_id" json:"group_id"`
	SecurityProtocol string `yaml:"security_protocol" json:"security_protocol"`
	SASLMechanism    string `yaml:"sasl_mechanism" json:"sasl_mechanism"`
	SASLUsername     string `yaml:"sasl_username" json:"sasl_username"`
	SASLPassword     string `yaml:"sasl_password" json:"sasl_password"`
}

// KafkaEventBus implements EventBus backed by Apache Kafka.
type KafkaEventBus struct {
	config        KafkaConfig
	mu            sync.RWMutex
	handlers      map[string]func(*engine.TelemetryEvent)
	published     int64
	consumed      int64
	errors        int64
	lastRateCheck time.Time
	lastPublished int64
	lastConsumed  int64
	closed        bool
	cancel        context.CancelFunc
}

// NewKafkaEventBus creates a new Kafka-backed event bus.
func NewKafkaEventBus(config KafkaConfig) (*KafkaEventBus, error) {
	if config.BootstrapServers == "" {
		return nil, fmt.Errorf("bootstrap_servers is required")
	}
	if config.GroupID == "" {
		config.GroupID = "kronveil-agent"
	}

	bus := &KafkaEventBus{
		config:        config,
		handlers:      make(map[string]func(*engine.TelemetryEvent)),
		lastRateCheck: time.Now(),
	}

	log.Printf("[eventbus] Kafka event bus initialized (servers: %s, group: %s)",
		config.BootstrapServers, config.GroupID)
	return bus, nil
}

// Publish sends a telemetry event to a Kafka topic.
func (b *KafkaEventBus) Publish(ctx context.Context, topic string, event *engine.TelemetryEvent) error {
	if b.closed {
		return fmt.Errorf("event bus is closed")
	}

	data, err := json.Marshal(event)
	if err != nil {
		atomic.AddInt64(&b.errors, 1)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// In production, this would use confluent-kafka-go Producer.Produce().
	// For now, we route directly to registered handlers (in-process mode).
	_ = data

	atomic.AddInt64(&b.published, 1)

	// Route to any registered subscribers for this topic.
	b.mu.RLock()
	handler, ok := b.handlers[topic]
	b.mu.RUnlock()

	if ok {
		handler(event)
		atomic.AddInt64(&b.consumed, 1)
	}

	return nil
}

// Subscribe registers a handler for events on a topic.
func (b *KafkaEventBus) Subscribe(ctx context.Context, topic string, group string, handler func(*engine.TelemetryEvent)) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("event bus is closed")
	}

	b.handlers[topic] = handler
	log.Printf("[eventbus] Subscribed to topic %s (group: %s)", topic, group)
	return nil
}

// Unsubscribe removes the handler for a topic.
func (b *KafkaEventBus) Unsubscribe(topic string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.handlers, topic)
	return nil
}

// Close shuts down the event bus.
func (b *KafkaEventBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	if b.cancel != nil {
		b.cancel()
	}
	log.Println("[eventbus] Kafka event bus closed")
	return nil
}

// Metrics returns current throughput metrics.
func (b *KafkaEventBus) Metrics() BusMetrics {
	now := time.Now()
	elapsed := now.Sub(b.lastRateCheck).Seconds()

	published := atomic.LoadInt64(&b.published)
	consumed := atomic.LoadInt64(&b.consumed)

	var publishRate, consumeRate float64
	if elapsed > 0 {
		publishRate = float64(published-b.lastPublished) / elapsed
		consumeRate = float64(consumed-b.lastConsumed) / elapsed
	}

	b.lastRateCheck = now
	b.lastPublished = published
	b.lastConsumed = consumed

	return BusMetrics{
		PublishedTotal: published,
		ConsumedTotal:  consumed,
		PublishRate:    publishRate,
		ConsumeRate:    consumeRate,
		ErrorsTotal:   atomic.LoadInt64(&b.errors),
	}
}
