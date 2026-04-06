// Package federation provides multi-cluster monitoring support.
package federation

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kronveil/kronveil/collectors/kubernetes"
	"github.com/kronveil/kronveil/core/engine"
)

// ClusterConfig holds the configuration for a single Kubernetes cluster.
type ClusterConfig struct {
	Name           string        `yaml:"name" json:"name"`
	KubeconfigPath string        `yaml:"kubeconfig_path" json:"kubeconfig_path"`
	Context        string        `yaml:"context" json:"context"`
	APIServer      string        `yaml:"api_server" json:"api_server"`
	Namespaces     []string      `yaml:"namespaces" json:"namespaces"`
	PollInterval   time.Duration `yaml:"poll_interval" json:"poll_interval"`
	Enabled        bool          `yaml:"enabled" json:"enabled"`
	Region         string        `yaml:"region" json:"region"`
}

// clusterState tracks the runtime state of a single cluster's collector.
type clusterState struct {
	config    ClusterConfig
	collector *kubernetes.Collector
	cancel    context.CancelFunc
}

// Manager manages multiple Kubernetes cluster collectors and provides a unified
// event stream. It implements the engine.Collector interface so it can be
// registered with the engine like any other collector.
type Manager struct {
	mu         sync.RWMutex
	clusters   map[string]*clusterState
	events     chan *engine.TelemetryEvent
	aggregator *Aggregator
	running    bool
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewManager creates a new federation manager.
func NewManager(clusters []ClusterConfig) *Manager {
	m := &Manager{
		clusters:   make(map[string]*clusterState),
		events:     make(chan *engine.TelemetryEvent, 4096),
		aggregator: NewAggregator(),
	}
	for _, cc := range clusters {
		if cc.Enabled {
			m.clusters[cc.Name] = &clusterState{config: cc}
		}
	}
	return m
}

// Name returns the collector name.
func (m *Manager) Name() string { return "federation" }

// Start begins collecting events from all configured clusters.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("federation manager already running")
	}
	m.running = true
	ctx, m.cancel = context.WithCancel(ctx)
	m.mu.Unlock()

	log.Printf("[federation] Starting federation manager with %d clusters", len(m.clusters))

	m.mu.RLock()
	for name, cs := range m.clusters {
		if err := m.startCluster(ctx, name, cs); err != nil {
			log.Printf("[federation] Failed to start cluster %s: %v", name, err)
		}
	}
	m.mu.RUnlock()

	log.Println("[federation] Federation manager started")
	return nil
}

// startCluster creates a collector for a cluster and begins forwarding its events.
// Caller must hold at least a read lock on m.mu.
func (m *Manager) startCluster(parentCtx context.Context, name string, cs *clusterState) error {
	cfg := kubernetes.Config{
		Kubeconfig:      cs.config.KubeconfigPath,
		Namespaces:      cs.config.Namespaces,
		PollInterval:    cs.config.PollInterval,
		MetricsInterval: cs.config.PollInterval,
		WatchEvents:     true,
		CollectMetrics:  true,
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 30 * time.Second
		cfg.MetricsInterval = 15 * time.Second
	}

	col := kubernetes.New(cfg)
	cs.collector = col

	clusterCtx, clusterCancel := context.WithCancel(parentCtx)
	cs.cancel = clusterCancel

	if err := col.Start(clusterCtx); err != nil {
		clusterCancel()
		return fmt.Errorf("start collector for cluster %s: %w", name, err)
	}

	m.wg.Add(1)
	go m.forwardEvents(clusterCtx, name, cs.config.Region, col)

	log.Printf("[federation] Cluster %s started", name)
	return nil
}

// forwardEvents reads events from a cluster collector, tags them with cluster
// metadata, deduplicates via the aggregator, and forwards to the unified channel.
func (m *Manager) forwardEvents(ctx context.Context, clusterName, region string, col *kubernetes.Collector) {
	defer m.wg.Done()
	ch := col.Events()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			// Tag with cluster metadata.
			event.Cluster = clusterName
			if event.Metadata == nil {
				event.Metadata = make(map[string]string)
			}
			event.Metadata["cluster_name"] = clusterName
			event.Metadata["cluster_region"] = region

			// Update aggregate metrics and deduplicate.
			if !m.aggregator.Process(event) {
				continue // duplicate
			}

			select {
			case m.events <- event:
			default:
				log.Printf("[federation] Event channel full, dropping event from cluster %s", clusterName)
			}
		}
	}
}

// Stop gracefully shuts down all cluster collectors.
func (m *Manager) Stop() error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = false
	m.mu.Unlock()

	log.Println("[federation] Shutting down federation manager...")

	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()

	m.mu.RLock()
	for name, cs := range m.clusters {
		if cs.collector != nil {
			if err := cs.collector.Stop(); err != nil {
				log.Printf("[federation] Error stopping collector for cluster %s: %v", name, err)
			}
		}
	}
	m.mu.RUnlock()

	close(m.events)
	log.Println("[federation] Federation manager stopped")
	return nil
}

// Events returns the unified event channel across all clusters.
func (m *Manager) Events() <-chan *engine.TelemetryEvent { return m.events }

// Health returns the aggregate health status across all managed clusters.
func (m *Manager) Health() engine.ComponentHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := len(m.clusters)
	healthy := 0
	var degradedMsg string

	for name, cs := range m.clusters {
		if cs.collector == nil {
			degradedMsg = fmt.Sprintf("cluster %s has no collector", name)
			continue
		}
		h := cs.collector.Health()
		if h.Status == "healthy" {
			healthy++
		} else {
			degradedMsg = fmt.Sprintf("cluster %s: %s", name, h.Message)
		}
	}

	status := "healthy"
	msg := fmt.Sprintf("%d/%d clusters healthy", healthy, total)
	if healthy < total {
		status = "degraded"
		msg = degradedMsg
	}
	if total == 0 {
		status = "degraded"
		msg = "no clusters configured"
	}

	return engine.ComponentHealth{
		Name:      "federation",
		Status:    status,
		Message:   msg,
		LastCheck: time.Now(),
	}
}

// AddCluster registers and starts a new cluster at runtime.
func (m *Manager) AddCluster(ctx context.Context, cfg ClusterConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clusters[cfg.Name]; exists {
		return fmt.Errorf("cluster %q already exists", cfg.Name)
	}

	cs := &clusterState{config: cfg}
	m.clusters[cfg.Name] = cs

	if m.running {
		if err := m.startCluster(ctx, cfg.Name, cs); err != nil {
			delete(m.clusters, cfg.Name)
			return err
		}
	}
	return nil
}

// RemoveCluster stops and removes a cluster at runtime.
func (m *Manager) RemoveCluster(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cs, exists := m.clusters[name]
	if !exists {
		return fmt.Errorf("cluster %q not found", name)
	}

	if cs.cancel != nil {
		cs.cancel()
	}
	if cs.collector != nil {
		if err := cs.collector.Stop(); err != nil {
			log.Printf("[federation] Error stopping collector for cluster %s: %v", name, err)
		}
	}
	delete(m.clusters, name)
	log.Printf("[federation] Cluster %s removed", name)
	return nil
}

// ListClusters returns the names of all registered clusters.
func (m *Manager) ListClusters() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.clusters))
	for name := range m.clusters {
		names = append(names, name)
	}
	return names
}

// ClusterHealth returns the health status for a specific cluster.
func (m *Manager) ClusterHealth(name string) (engine.ComponentHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cs, exists := m.clusters[name]
	if !exists {
		return engine.ComponentHealth{}, fmt.Errorf("cluster %q not found", name)
	}
	if cs.collector == nil {
		return engine.ComponentHealth{
			Name:      name,
			Status:    "degraded",
			Message:   "collector not initialized",
			LastCheck: time.Now(),
		}, nil
	}
	return cs.collector.Health(), nil
}

// Aggregator returns the federation aggregator for querying aggregate metrics.
func (m *Manager) Aggregator() *Aggregator {
	return m.aggregator
}
