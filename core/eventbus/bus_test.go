package eventbus

import (
	"context"
	"testing"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

func TestNewKafkaEventBus_EmptyServers(t *testing.T) {
	_, err := NewKafkaEventBus(KafkaConfig{})
	if err == nil {
		t.Error("expected error for empty bootstrap_servers")
	}
}

func TestNewKafkaEventBus_DefaultGroupID(t *testing.T) {
	bus, err := NewKafkaEventBus(KafkaConfig{BootstrapServers: "localhost:9092"})
	if err != nil {
		t.Fatalf("NewKafkaEventBus failed: %v", err)
	}
	if bus.config.GroupID != "kronveil-agent" {
		t.Errorf("expected default groupID 'kronveil-agent', got %s", bus.config.GroupID)
	}
}

func TestPublishSubscribe(t *testing.T) {
	bus, _ := NewKafkaEventBus(KafkaConfig{BootstrapServers: "localhost:9092"})

	var received *engine.TelemetryEvent
	err := bus.Subscribe(context.Background(), "test-topic", "group", func(e *engine.TelemetryEvent) {
		received = e
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	event := &engine.TelemetryEvent{
		ID:     "evt-1",
		Source: "test",
	}
	err = bus.Publish(context.Background(), "test-topic", event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if received == nil || received.ID != "evt-1" {
		t.Error("subscriber should receive the published event")
	}
}

func TestUnsubscribe(t *testing.T) {
	bus, _ := NewKafkaEventBus(KafkaConfig{BootstrapServers: "localhost:9092"})

	called := false
	_ = bus.Subscribe(context.Background(), "topic", "group", func(e *engine.TelemetryEvent) {
		called = true
	})
	_ = bus.Unsubscribe("topic")

	_ = bus.Publish(context.Background(), "topic", &engine.TelemetryEvent{ID: "e1"})
	if called {
		t.Error("handler should not be called after unsubscribe")
	}
}

func TestMetrics_Counters(t *testing.T) {
	bus, _ := NewKafkaEventBus(KafkaConfig{BootstrapServers: "localhost:9092"})

	_ = bus.Subscribe(context.Background(), "t", "g", func(*engine.TelemetryEvent) {})
	_ = bus.Publish(context.Background(), "t", &engine.TelemetryEvent{})
	_ = bus.Publish(context.Background(), "t", &engine.TelemetryEvent{})

	m := bus.Metrics()
	if m.PublishedTotal != 2 {
		t.Errorf("PublishedTotal = %d, want 2", m.PublishedTotal)
	}
	if m.ConsumedTotal != 2 {
		t.Errorf("ConsumedTotal = %d, want 2", m.ConsumedTotal)
	}
}

func TestClosedBus_RejectsOperations(t *testing.T) {
	bus, _ := NewKafkaEventBus(KafkaConfig{BootstrapServers: "localhost:9092"})
	_ = bus.Close()

	err := bus.Publish(context.Background(), "t", &engine.TelemetryEvent{})
	if err == nil {
		t.Error("expected error publishing to closed bus")
	}

	err = bus.Subscribe(context.Background(), "t", "g", func(*engine.TelemetryEvent) {})
	if err == nil {
		t.Error("expected error subscribing to closed bus")
	}
}

func TestMetrics_Rates(t *testing.T) {
	bus, _ := NewKafkaEventBus(KafkaConfig{BootstrapServers: "localhost:9092"})

	_ = bus.Subscribe(context.Background(), "t", "g", func(*engine.TelemetryEvent) {})

	// Publish a few events.
	for i := 0; i < 5; i++ {
		_ = bus.Publish(context.Background(), "t", &engine.TelemetryEvent{})
	}

	time.Sleep(10 * time.Millisecond)
	m := bus.Metrics()
	if m.PublishRate <= 0 {
		t.Error("expected positive publish rate")
	}
}
