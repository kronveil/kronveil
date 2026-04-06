package federation

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// AggregateMetrics holds aggregate counts across all clusters.
type AggregateMetrics struct {
	TotalPods   int       `json:"total_pods"`
	TotalNodes  int       `json:"total_nodes"`
	TotalEvents int64     `json:"total_events"`
	LastUpdated time.Time `json:"last_updated"`
}

// Aggregator merges events from multiple cluster collectors, deduplicates
// cross-cluster events, and tracks aggregate metrics.
type Aggregator struct {
	mu      sync.RWMutex
	seen    map[string]time.Time // event fingerprint -> last seen time
	metrics AggregateMetrics

	// dedup window: events with the same fingerprint within this window are dropped.
	dedupWindow time.Duration
}

// NewAggregator creates a new event aggregator with a default dedup window.
func NewAggregator() *Aggregator {
	return &Aggregator{
		seen:        make(map[string]time.Time),
		dedupWindow: 30 * time.Second,
	}
}

// Process adds cluster metadata to an event, checks for duplicates, and updates
// aggregate metrics. Returns true if the event is new and should be forwarded.
func (a *Aggregator) Process(event *engine.TelemetryEvent) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Update aggregate metrics based on event type.
	a.metrics.TotalEvents++
	a.metrics.LastUpdated = time.Now()
	a.updateMetricsFromEvent(event)

	// Deduplicate using a fingerprint of source + type + key payload fields.
	fp := a.fingerprint(event)
	if lastSeen, exists := a.seen[fp]; exists {
		if time.Since(lastSeen) < a.dedupWindow {
			return false
		}
	}
	a.seen[fp] = time.Now()

	// Periodically prune old fingerprints to prevent unbounded growth.
	if len(a.seen) > 10000 {
		a.prune()
	}

	return true
}

// Metrics returns a snapshot of the current aggregate metrics.
func (a *Aggregator) Metrics() AggregateMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.metrics
}

// fingerprint computes a dedup key for an event based on its core fields.
func (a *Aggregator) fingerprint(event *engine.TelemetryEvent) string {
	raw := fmt.Sprintf("%s:%s:%s", event.Source, event.Type, event.Cluster)

	// Include identifying payload fields when present.
	for _, key := range []string{"pod", "node", "namespace", "reason"} {
		if v, ok := event.Payload[key]; ok {
			raw += fmt.Sprintf(":%v", v)
		}
	}

	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash[:8])
}

// updateMetricsFromEvent adjusts aggregate counters based on event type.
func (a *Aggregator) updateMetricsFromEvent(event *engine.TelemetryEvent) {
	switch event.Type {
	case "pod_status", "pod_metrics":
		if v, ok := event.Payload["pods_total"]; ok {
			if n, ok := v.(int); ok {
				a.metrics.TotalPods = n
			}
		}
	case "node_metrics":
		if v, ok := event.Payload["nodes_total"]; ok {
			if n, ok := v.(int); ok {
				a.metrics.TotalNodes = n
			}
		}
	}
}

// prune removes fingerprints older than the dedup window.
func (a *Aggregator) prune() {
	cutoff := time.Now().Add(-a.dedupWindow)
	for fp, ts := range a.seen {
		if ts.Before(cutoff) {
			delete(a.seen, fp)
		}
	}
}
