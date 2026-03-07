package prometheus

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// Config holds Prometheus exporter configuration.
type Config struct {
	Port         int    `yaml:"port" json:"port"`
	MetricsPath  string `yaml:"metrics_path" json:"metrics_path"`
	Namespace    string `yaml:"namespace" json:"namespace"`
}

// DefaultConfig returns default Prometheus configuration.
func DefaultConfig() Config {
	return Config{
		Port:        9090,
		MetricsPath: "/metrics",
		Namespace:   "kronveil",
	}
}

// Exporter exposes Kronveil metrics for Prometheus scraping.
type Exporter struct {
	config  Config
	metrics *MetricsSet
	server  *http.Server
}

// MetricsSet holds all Kronveil Prometheus metrics.
type MetricsSet struct {
	// Collector metrics
	EventsTotal        map[string]int64 // source -> count
	EventsPerSec       map[string]float64
	CollectorErrors     map[string]int64

	// Intelligence metrics
	AnomaliesDetected   int64
	IncidentsCreated    int64
	IncidentsResolved   int64
	RemediationsTotal   int64
	RemediationsSuccess int64
	RemediationsFailed  int64
	MTTRSeconds         float64

	// Policy metrics
	PolicyEvaluations   int64
	PolicyViolations    int64

	// System metrics
	UptimeSeconds       float64
	ComponentsHealthy   int
	ComponentsDegraded  int
	ComponentsCritical  int
}

// NewExporter creates a new Prometheus metrics exporter.
func NewExporter(config Config) *Exporter {
	return &Exporter{
		config: config,
		metrics: &MetricsSet{
			EventsTotal:   make(map[string]int64),
			EventsPerSec:  make(map[string]float64),
			CollectorErrors: make(map[string]int64),
		},
	}
}

func (e *Exporter) Name() string { return "prometheus" }

func (e *Exporter) Initialize(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc(e.config.MetricsPath, e.metricsHandler)

	e.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", e.config.Port),
		Handler: mux,
	}

	go func() {
		log.Printf("[prometheus] Metrics server listening on :%d%s",
			e.config.Port, e.config.MetricsPath)
		if err := e.server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("[prometheus] Metrics server error: %v", err)
		}
	}()

	return nil
}

func (e *Exporter) Close() error {
	if e.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return e.server.Shutdown(ctx)
	}
	return nil
}

func (e *Exporter) Health() engine.ComponentHealth {
	return engine.ComponentHealth{
		Name:      "prometheus-exporter",
		Status:    "healthy",
		Message:   fmt.Sprintf("serving metrics on :%d", e.config.Port),
		LastCheck: time.Now(),
	}
}

// RecordEvent records a telemetry event metric.
func (e *Exporter) RecordEvent(source string) {
	e.metrics.EventsTotal[source]++
}

// RecordAnomaly increments the anomaly counter.
func (e *Exporter) RecordAnomaly() {
	e.metrics.AnomaliesDetected++
}

// RecordIncident increments the incident counter.
func (e *Exporter) RecordIncident() {
	e.metrics.IncidentsCreated++
}

// RecordRemediation records a remediation outcome.
func (e *Exporter) RecordRemediation(success bool) {
	e.metrics.RemediationsTotal++
	if success {
		e.metrics.RemediationsSuccess++
	} else {
		e.metrics.RemediationsFailed++
	}
}

// SetMTTR updates the average MTTR metric.
func (e *Exporter) SetMTTR(seconds float64) {
	e.metrics.MTTRSeconds = seconds
}

func (e *Exporter) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	ns := e.config.Namespace

	// Collector metrics
	for source, count := range e.metrics.EventsTotal {
		fmt.Fprintf(w, "%s_events_total{source=\"%s\"} %d\n", ns, source, count)
	}
	for source, rate := range e.metrics.EventsPerSec {
		fmt.Fprintf(w, "%s_events_per_second{source=\"%s\"} %.2f\n", ns, source, rate)
	}

	// Intelligence metrics
	fmt.Fprintf(w, "%s_anomalies_detected_total %d\n", ns, e.metrics.AnomaliesDetected)
	fmt.Fprintf(w, "%s_incidents_created_total %d\n", ns, e.metrics.IncidentsCreated)
	fmt.Fprintf(w, "%s_incidents_resolved_total %d\n", ns, e.metrics.IncidentsResolved)
	fmt.Fprintf(w, "%s_remediations_total %d\n", ns, e.metrics.RemediationsTotal)
	fmt.Fprintf(w, "%s_remediations_success_total %d\n", ns, e.metrics.RemediationsSuccess)
	fmt.Fprintf(w, "%s_remediations_failed_total %d\n", ns, e.metrics.RemediationsFailed)
	fmt.Fprintf(w, "%s_mttr_seconds %.2f\n", ns, e.metrics.MTTRSeconds)

	// Policy metrics
	fmt.Fprintf(w, "%s_policy_evaluations_total %d\n", ns, e.metrics.PolicyEvaluations)
	fmt.Fprintf(w, "%s_policy_violations_total %d\n", ns, e.metrics.PolicyViolations)

	// System metrics
	fmt.Fprintf(w, "%s_uptime_seconds %.0f\n", ns, e.metrics.UptimeSeconds)
	fmt.Fprintf(w, "%s_components_healthy %d\n", ns, e.metrics.ComponentsHealthy)
	fmt.Fprintf(w, "%s_components_degraded %d\n", ns, e.metrics.ComponentsDegraded)
	fmt.Fprintf(w, "%s_components_critical %d\n", ns, e.metrics.ComponentsCritical)
}
