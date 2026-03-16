// Package kubernetes collects pod, node, and event telemetry from Kubernetes clusters.
package kubernetes

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
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
	config        Config
	clientset     kubernetes.Interface
	metricsClient *metricsclientset.Clientset
	events        chan *engine.TelemetryEvent
	mu            sync.RWMutex
	running       bool
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	lastErr       error
	stats         collectorStats
}

type collectorStats struct {
	podsWatched   int
	nodesWatched  int
	eventsEmitted int64
	lastEventTime time.Time
}

// New creates a new Kubernetes collector.
func New(config Config) *Collector {
	c := &Collector{
		config: config,
		events: make(chan *engine.TelemetryEvent, 1000),
	}

	// Try to build Kubernetes client config.
	var restConfig *rest.Config
	var err error
	if config.Kubeconfig != "" {
		restConfig, err = clientcmd.BuildConfigFromFlags("", config.Kubeconfig)
	} else {
		restConfig, err = rest.InClusterConfig()
	}

	if err != nil {
		log.Printf("[k8s-collector] WARNING: failed to build k8s config: %v (running in degraded mode)", err)
		return c
	}

	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Printf("[k8s-collector] WARNING: failed to create k8s clientset: %v (running in degraded mode)", err)
		return c
	}
	c.clientset = cs

	mc, err := metricsclientset.NewForConfig(restConfig)
	if err != nil {
		log.Printf("[k8s-collector] WARNING: failed to create metrics clientset: %v", err)
	} else {
		c.metricsClient = mc
	}

	return c
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
	_, c.cancel = context.WithCancel(ctx)
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
	if c.clientset == nil {
		status = "degraded"
		msg = "no kubernetes client configured"
	} else if c.lastErr != nil {
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

	if c.clientset == nil {
		// Fallback: emit stub events on a timer.
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

	namespaces := c.config.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{""}
	}

	for _, ns := range namespaces {
		watcher, err := c.clientset.CoreV1().Pods(ns).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			c.mu.Lock()
			c.lastErr = err
			c.mu.Unlock()
			log.Printf("[k8s-collector] Failed to watch pods in %q: %v", ns, err)
			continue
		}
		defer watcher.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.ResultChan():
				if !ok {
					return
				}
				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					continue
				}

				c.mu.Lock()
				c.stats.podsWatched++
				c.mu.Unlock()

				for _, cs := range pod.Status.ContainerStatuses {
					if cs.State.Waiting != nil {
						severity := engine.SeverityInfo
						reason := cs.State.Waiting.Reason
						switch reason {
						case "CrashLoopBackOff":
							severity = engine.SeverityCritical
						case "OOMKilled":
							severity = engine.SeverityCritical
						case "ImagePullBackOff", "ErrImagePull":
							severity = engine.SeverityHigh
						}
						if severity != engine.SeverityInfo {
							c.emitEvent("pod_issue", map[string]interface{}{
								"pod":       pod.Name,
								"namespace": pod.Namespace,
								"container": cs.Name,
								"reason":    reason,
								"message":   cs.State.Waiting.Message,
								"restarts":  cs.RestartCount,
							}, severity)
						}
					}
				}
			}
		}
	}
}

func (c *Collector) watchEvents(ctx context.Context) {
	defer c.wg.Done()

	if c.clientset == nil {
		// No client; wait for context cancellation.
		<-ctx.Done()
		return
	}

	namespaces := c.config.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{""}
	}

	for _, ns := range namespaces {
		watcher, err := c.clientset.CoreV1().Events(ns).Watch(ctx, metav1.ListOptions{
			FieldSelector: "type=Warning",
		})
		if err != nil {
			c.mu.Lock()
			c.lastErr = err
			c.mu.Unlock()
			log.Printf("[k8s-collector] Failed to watch events in %q: %v", ns, err)
			continue
		}
		defer watcher.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.ResultChan():
				if !ok {
					return
				}
				k8sEvent, ok := event.Object.(*corev1.Event)
				if !ok {
					continue
				}

				severity := engine.SeverityMedium
				switch k8sEvent.Reason {
				case "OOMKilling", "OOMKilled":
					severity = engine.SeverityCritical
				case "FailedScheduling", "FailedMount", "FailedAttachVolume":
					severity = engine.SeverityHigh
				case "BackOff":
					severity = engine.SeverityHigh
				case "Unhealthy", "ProbeWarning":
					severity = engine.SeverityMedium
				}

				c.emitEvent("k8s_event", map[string]interface{}{
					"reason":    k8sEvent.Reason,
					"message":   k8sEvent.Message,
					"namespace": k8sEvent.Namespace,
					"object":    k8sEvent.InvolvedObject.Name,
					"kind":      k8sEvent.InvolvedObject.Kind,
					"count":     k8sEvent.Count,
				}, severity)
			}
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
			if c.metricsClient == nil {
				c.emitEvent("node_metrics", map[string]interface{}{
					"type":        "node_resource_usage",
					"nodes_total": c.stats.nodesWatched,
				}, engine.SeverityInfo)
				continue
			}

			// Collect node metrics.
			nodeMetrics, err := c.metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
			if err != nil {
				c.mu.Lock()
				c.lastErr = err
				c.mu.Unlock()
				log.Printf("[k8s-collector] Failed to get node metrics: %v", err)
				continue
			}

			c.mu.Lock()
			c.stats.nodesWatched = len(nodeMetrics.Items)
			c.mu.Unlock()

			for _, nm := range nodeMetrics.Items {
				cpuMillis := nm.Usage.Cpu().MilliValue()
				memBytes := nm.Usage.Memory().Value()
				c.emitEvent("node_metrics", map[string]interface{}{
					"node":       nm.Name,
					"cpu_millis": cpuMillis,
					"mem_bytes":  memBytes,
				}, engine.SeverityInfo)
			}

			// Collect pod metrics per namespace.
			namespaces := c.config.Namespaces
			if len(namespaces) == 0 {
				namespaces = []string{""}
			}
			for _, ns := range namespaces {
				podMetrics, err := c.metricsClient.MetricsV1beta1().PodMetricses(ns).List(ctx, metav1.ListOptions{})
				if err != nil {
					log.Printf("[k8s-collector] Failed to get pod metrics in %q: %v", ns, err)
					continue
				}
				for _, pm := range podMetrics.Items {
					for _, container := range pm.Containers {
						cpuMillis := container.Usage.Cpu().MilliValue()
						memBytes := container.Usage.Memory().Value()
						c.emitEvent("pod_metrics", map[string]interface{}{
							"pod":        pm.Name,
							"namespace":  pm.Namespace,
							"container":  container.Name,
							"cpu_millis": cpuMillis,
							"mem_bytes":  memBytes,
						}, engine.SeverityInfo)
					}
				}
			}
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
