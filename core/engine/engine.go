// Package engine provides the central orchestration engine for Kronveil.
package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Engine is the main Kronveil agent engine that orchestrates all components.
type Engine struct {
	registry  *Registry
	publisher EventPublisher
	metrics   MetricsRecorder
	startTime time.Time
	mu        sync.RWMutex
	running   bool
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewEngine creates a new agent engine.
func NewEngine(registry *Registry, publisher EventPublisher, metrics MetricsRecorder) *Engine {
	if metrics == nil {
		metrics = &NoopMetricsRecorder{}
	}
	return &Engine{
		registry:  registry,
		publisher: publisher,
		metrics:   metrics,
	}
}

// Start initializes and starts all registered components.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("engine already running")
	}
	e.running = true
	e.startTime = time.Now()
	_, e.cancel = context.WithCancel(ctx)
	e.mu.Unlock()

	log.Println("[engine] Starting Kronveil agent engine...")

	// Initialize integrations first since other components may depend on them.
	for _, intg := range e.registry.Integrations() {
		log.Printf("[engine] Initializing integration: %s", intg.Name())
		if err := intg.Initialize(ctx); err != nil {
			return fmt.Errorf("failed to initialize integration %s: %w", intg.Name(), err)
		}
	}

	// Start intelligence modules so they're ready to receive events.
	for _, mod := range e.registry.Modules() {
		log.Printf("[engine] Starting intelligence module: %s", mod.Name())
		if err := mod.Start(ctx); err != nil {
			return fmt.Errorf("failed to start module %s: %w", mod.Name(), err)
		}
	}

	// Start collectors and route their events.
	for _, col := range e.registry.Collectors() {
		log.Printf("[engine] Starting collector: %s", col.Name())
		if err := col.Start(ctx); err != nil {
			return fmt.Errorf("failed to start collector %s: %w", col.Name(), err)
		}
		e.wg.Add(1)
		go e.routeEvents(ctx, col)
	}

	log.Println("[engine] Kronveil agent engine started successfully")
	return nil
}

// routeEvents reads events from a collector and fans them out to intelligence modules.
func (e *Engine) routeEvents(ctx context.Context, col Collector) {
	defer e.wg.Done()
	events := col.Events()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}

			e.metrics.RecordEvent(event.Source)

			// Publish to event bus for persistence and streaming.
			if e.publisher != nil {
				topic := fmt.Sprintf("kronveil.telemetry.%s", event.Source)
				if err := e.publisher.Publish(ctx, topic, event); err != nil {
					log.Printf("[engine] Failed to publish event to bus: %v", err)
					e.metrics.RecordCollectorError(event.Source)
				}
			}

			// Fan out to all intelligence modules for analysis.
			for _, mod := range e.registry.Modules() {
				if err := mod.Analyze(ctx, event); err != nil {
					log.Printf("[engine] Module %s analysis error: %v", mod.Name(), err)
				}
			}
		}
	}
}

// Stop gracefully shuts down all components.
func (e *Engine) Stop() error {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return nil
	}
	e.running = false
	e.mu.Unlock()

	log.Println("[engine] Shutting down Kronveil agent engine...")

	// Cancel context to signal all goroutines.
	if e.cancel != nil {
		e.cancel()
	}

	// Wait for event routing goroutines to finish.
	e.wg.Wait()

	// Stop collectors.
	for _, col := range e.registry.Collectors() {
		log.Printf("[engine] Stopping collector: %s", col.Name())
		if err := col.Stop(); err != nil {
			log.Printf("[engine] Error stopping collector %s: %v", col.Name(), err)
		}
	}

	// Stop intelligence modules.
	for _, mod := range e.registry.Modules() {
		log.Printf("[engine] Stopping module: %s", mod.Name())
		if err := mod.Stop(); err != nil {
			log.Printf("[engine] Error stopping module %s: %v", mod.Name(), err)
		}
	}

	// Close integrations.
	for _, intg := range e.registry.Integrations() {
		log.Printf("[engine] Closing integration: %s", intg.Name())
		if err := intg.Close(); err != nil {
			log.Printf("[engine] Error closing integration %s: %v", intg.Name(), err)
		}
	}

	log.Println("[engine] Kronveil agent engine stopped")
	return nil
}

// Status returns the current health status of the engine and all components.
func (e *Engine) Status() HealthStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	status := "healthy"
	components := e.registry.Health()

	healthy, degraded, critical := 0, 0, 0
	for _, c := range components {
		switch c.Status {
		case "critical":
			status = "critical"
			critical++
		case "degraded":
			if status != "critical" {
				status = "degraded"
			}
			degraded++
		default:
			healthy++
		}
	}

	var uptime time.Duration
	if e.running {
		uptime = time.Since(e.startTime)
	}

	e.metrics.SetComponentHealth(healthy, degraded, critical)
	e.metrics.SetUptime(uptime.Seconds())

	return HealthStatus{
		Status:     status,
		Components: components,
		Uptime:     uptime,
	}
}

// Registry returns the component registry.
func (e *Engine) Registry() *Registry {
	return e.registry
}

// IsRunning returns whether the engine is currently running.
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}
