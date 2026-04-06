package collector_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kronveil/kronveil/sdk/collector"
)

// httpChecker is an example Plugin that checks HTTP endpoint availability.
type httpChecker struct {
	url string
}

func (h *httpChecker) Name() string { return "http-checker" }

func (h *httpChecker) Collect(ctx context.Context) ([]*collector.Event, error) {
	// In a real implementation you would make an HTTP request to h.url and
	// report latency, status code, etc. This example returns a static event.
	return []*collector.Event{
		{
			Type:     "http_check",
			Severity: "info",
			Payload: map[string]interface{}{
				"url":         h.url,
				"status_code": 200,
				"latency_ms":  42,
			},
			Timestamp: time.Now(),
		},
	}, nil
}

func (h *httpChecker) Healthcheck(ctx context.Context) error {
	return nil
}

// TestHTTPCheckerPlugin verifies that the httpChecker plugin can be wrapped
// by the SDK Builder into a working engine.Collector.
func TestHTTPCheckerPlugin(t *testing.T) {
	plugin := &httpChecker{url: "https://example.com/health"}

	col := collector.NewBuilder(plugin).
		WithPollInterval(100 * time.Millisecond).
		WithBufferSize(16).
		Build()

	if col.Name() != "http-checker" {
		t.Fatalf("expected name http-checker, got %s", col.Name())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := col.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Wait for at least one event from the polling loop.
	select {
	case evt, ok := <-col.Events():
		if !ok {
			t.Fatal("events channel closed unexpectedly")
		}
		if evt.Source != "http-checker" {
			t.Errorf("expected source http-checker, got %s", evt.Source)
		}
		if evt.Type != "http_check" {
			t.Errorf("expected type http_check, got %s", evt.Type)
		}
		statusCode, ok := evt.Payload["status_code"]
		if !ok || statusCode != 200 {
			t.Errorf("unexpected payload: %v", evt.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}

	// Health should report healthy.
	health := col.Health()
	if health.Status != "healthy" {
		t.Errorf("expected healthy status, got %s: %s", health.Status, health.Message)
	}

	if err := col.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
}

// failingPlugin demonstrates error handling in the SDK adapter.
type failingPlugin struct{}

func (f *failingPlugin) Name() string { return "failing-plugin" }
func (f *failingPlugin) Collect(ctx context.Context) ([]*collector.Event, error) {
	return nil, fmt.Errorf("simulated failure")
}
func (f *failingPlugin) Healthcheck(ctx context.Context) error { return nil }

func TestFailingPluginHealth(t *testing.T) {
	col := collector.NewBuilder(&failingPlugin{}).
		WithPollInterval(50 * time.Millisecond).
		WithBufferSize(8).
		Build()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := col.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Give the adapter time to attempt a collect and record the error.
	time.Sleep(200 * time.Millisecond)

	health := col.Health()
	if health.Status != "degraded" {
		t.Errorf("expected degraded status after failure, got %s: %s", health.Status, health.Message)
	}

	if err := col.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
}
