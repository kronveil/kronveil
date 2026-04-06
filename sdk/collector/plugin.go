// Package collector provides the SDK for building custom Kronveil collectors.
//
// Instead of implementing the full engine.Collector interface directly, users
// implement the simplified Plugin interface and use the Builder to produce a
// ready-to-register engine.Collector.
//
// Example usage:
//
//	col := collector.NewBuilder(&myPlugin{}).
//		WithPollInterval(10 * time.Second).
//		WithBufferSize(256).
//		Build()
//	registry.RegisterCollector(col)
package collector

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// Plugin is the simplified interface that custom collector authors implement.
// It abstracts away the lifecycle plumbing (goroutines, channels, health reporting)
// so users only need to focus on data collection logic.
type Plugin interface {
	// Name returns a unique identifier for this collector plugin.
	Name() string

	// Collect is called on each poll interval. It should return zero or more
	// events gathered during this cycle. Return a non-nil error to signal a
	// collection failure (the adapter will log it and continue polling).
	Collect(ctx context.Context) ([]*Event, error)

	// Healthcheck performs an optional health check. Return nil if healthy.
	// Implementations that do not need health checking can simply return nil.
	Healthcheck(ctx context.Context) error
}

// Event is a simplified telemetry event that plugin authors produce.
// The adapter converts these to engine.TelemetryEvent before sending them
// through the collector's event channel.
type Event struct {
	Source    string
	Type      string
	Payload   map[string]interface{}
	Severity  string
	Timestamp time.Time
}

// Builder provides a fluent API for constructing an engine.Collector from a Plugin.
type Builder struct {
	plugin       Plugin
	pollInterval time.Duration
	bufferSize   int
	logger       *slog.Logger
}

// NewBuilder creates a new Builder for the given plugin.
func NewBuilder(plugin Plugin) *Builder {
	return &Builder{
		plugin:       plugin,
		pollInterval: 30 * time.Second,
		bufferSize:   256,
	}
}

// WithPollInterval sets the interval between calls to Plugin.Collect.
func (b *Builder) WithPollInterval(d time.Duration) *Builder {
	b.pollInterval = d
	return b
}

// WithBufferSize sets the capacity of the internal event channel.
func (b *Builder) WithBufferSize(n int) *Builder {
	b.bufferSize = n
	return b
}

// WithLogger sets a structured logger for the adapter. If nil, a default
// logger is used.
func (b *Builder) WithLogger(logger *slog.Logger) *Builder {
	b.logger = logger
	return b
}

// Build produces an engine.Collector that wraps the plugin.
func (b *Builder) Build() engine.Collector {
	logger := b.logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Adapter{
		plugin:       b.plugin,
		pollInterval: b.pollInterval,
		events:       make(chan *engine.TelemetryEvent, b.bufferSize),
		logger:       logger,
	}
}

// Adapter wraps a Plugin and implements the full engine.Collector interface.
// It manages the polling loop, event conversion, and lifecycle.
type Adapter struct {
	plugin       Plugin
	pollInterval time.Duration
	events       chan *engine.TelemetryEvent
	logger       *slog.Logger

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	lastErr     error
	lastErrTime time.Time
}

// Name implements engine.Collector.
func (a *Adapter) Name() string {
	return a.plugin.Name()
}

// Start implements engine.Collector. It launches the background polling loop.
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("collector %s already running", a.plugin.Name())
	}
	a.running = true

	pollCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	a.wg.Add(1)
	go a.pollLoop(pollCtx)

	a.logger.Info("collector started", "name", a.plugin.Name(), "interval", a.pollInterval)
	return nil
}

// Stop implements engine.Collector. It signals the polling loop to stop and
// waits for it to finish before closing the event channel.
func (a *Adapter) Stop() error {
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return nil
	}
	a.running = false
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()

	a.wg.Wait()
	close(a.events)

	a.logger.Info("collector stopped", "name", a.plugin.Name())
	return nil
}

// Events implements engine.Collector.
func (a *Adapter) Events() <-chan *engine.TelemetryEvent {
	return a.events
}

// Health implements engine.Collector.
func (a *Adapter) Health() engine.ComponentHealth {
	status := "healthy"
	message := "ok"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.plugin.Healthcheck(ctx); err != nil {
		status = "degraded"
		message = err.Error()
	}

	a.mu.Lock()
	lastErr := a.lastErr
	lastErrTime := a.lastErrTime
	a.mu.Unlock()

	if lastErr != nil && time.Since(lastErrTime) < 2*a.pollInterval {
		status = "degraded"
		message = fmt.Sprintf("last collect error: %v", lastErr)
	}

	return engine.ComponentHealth{
		Name:      a.plugin.Name(),
		Status:    status,
		Message:   message,
		LastCheck: time.Now(),
	}
}

// pollLoop runs Plugin.Collect at the configured interval and converts the
// resulting Events into engine.TelemetryEvents.
func (a *Adapter) pollLoop(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	// Do an initial collect immediately rather than waiting for the first tick.
	a.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.collect(ctx)
		}
	}
}

func (a *Adapter) collect(ctx context.Context) {
	events, err := a.plugin.Collect(ctx)
	if err != nil {
		a.logger.Error("collect failed", "name", a.plugin.Name(), "error", err)
		a.mu.Lock()
		a.lastErr = err
		a.lastErrTime = time.Now()
		a.mu.Unlock()
		return
	}

	for _, evt := range events {
		te := convertEvent(a.plugin.Name(), evt)
		select {
		case a.events <- te:
		default:
			a.logger.Warn("event channel full, dropping event", "name", a.plugin.Name())
		}
	}
}

// convertEvent maps an SDK Event to an engine.TelemetryEvent.
func convertEvent(pluginName string, evt *Event) *engine.TelemetryEvent {
	ts := evt.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	source := evt.Source
	if source == "" {
		source = pluginName
	}

	severity := evt.Severity
	if severity == "" {
		severity = engine.SeverityInfo
	}

	return &engine.TelemetryEvent{
		ID:        fmt.Sprintf("%s-%d", pluginName, time.Now().UnixNano()),
		Source:    source,
		Type:      evt.Type,
		Timestamp: ts,
		Payload:   evt.Payload,
		Metadata:  map[string]string{"collector": pluginName},
		Severity:  severity,
	}
}
