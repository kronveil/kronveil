package prometheus

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/kronveil/kronveil/core/engine"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config holds Prometheus exporter configuration.
type Config struct {
	Port        int
	MetricsPath string
}

// Exporter exposes Kronveil metrics for Prometheus scraping.
// It implements both engine.Integration and engine.MetricsRecorder.
type Exporter struct {
	config   Config
	registry *prometheus.Registry
	server   *http.Server

	// Counters
	eventsTotal        *prometheus.CounterVec
	collectorErrors    *prometheus.CounterVec
	anomaliesDetected  prometheus.Counter
	incidentsCreated   prometheus.Counter
	incidentsResolved  prometheus.Counter
	remediationsTotal  prometheus.Counter
	remediationsSuccess prometheus.Counter
	remediationsFailed prometheus.Counter
	policyEvaluations  prometheus.Counter
	policyViolations   prometheus.Counter

	// Gauges
	mttrSeconds        prometheus.Gauge
	uptimeSeconds      prometheus.Gauge
	componentsHealthy  prometheus.Gauge
	componentsDegraded prometheus.Gauge
	componentsCritical prometheus.Gauge
}

// NewExporter creates a new Prometheus metrics exporter.
func NewExporter(config Config) *Exporter {
	reg := prometheus.NewRegistry()

	e := &Exporter{
		config:   config,
		registry: reg,

		eventsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "kronveil",
			Name:      "events_total",
			Help:      "Total number of telemetry events received",
		}, []string{"source"}),

		collectorErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "kronveil",
			Name:      "collector_errors_total",
			Help:      "Total number of collector errors",
		}, []string{"source"}),

		anomaliesDetected: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "kronveil",
			Name:      "anomalies_detected_total",
			Help:      "Total number of anomalies detected",
		}),

		incidentsCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "kronveil",
			Name:      "incidents_created_total",
			Help:      "Total number of incidents created",
		}),

		incidentsResolved: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "kronveil",
			Name:      "incidents_resolved_total",
			Help:      "Total number of incidents resolved",
		}),

		remediationsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "kronveil",
			Name:      "remediations_total",
			Help:      "Total number of remediation actions attempted",
		}),

		remediationsSuccess: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "kronveil",
			Name:      "remediations_success_total",
			Help:      "Total number of successful remediations",
		}),

		remediationsFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "kronveil",
			Name:      "remediations_failed_total",
			Help:      "Total number of failed remediations",
		}),

		policyEvaluations: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "kronveil",
			Name:      "policy_evaluations_total",
			Help:      "Total number of policy evaluations",
		}),

		policyViolations: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "kronveil",
			Name:      "policy_violations_total",
			Help:      "Total number of policy violations detected",
		}),

		mttrSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "kronveil",
			Name:      "mttr_seconds",
			Help:      "Mean time to recovery in seconds",
		}),

		uptimeSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "kronveil",
			Name:      "uptime_seconds",
			Help:      "Agent uptime in seconds",
		}),

		componentsHealthy: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "kronveil",
			Name:      "components_healthy",
			Help:      "Number of healthy components",
		}),

		componentsDegraded: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "kronveil",
			Name:      "components_degraded",
			Help:      "Number of degraded components",
		}),

		componentsCritical: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "kronveil",
			Name:      "components_critical",
			Help:      "Number of critical components",
		}),
	}

	// Register all metrics with the custom registry.
	reg.MustRegister(
		e.eventsTotal,
		e.collectorErrors,
		e.anomaliesDetected,
		e.incidentsCreated,
		e.incidentsResolved,
		e.remediationsTotal,
		e.remediationsSuccess,
		e.remediationsFailed,
		e.policyEvaluations,
		e.policyViolations,
		e.mttrSeconds,
		e.uptimeSeconds,
		e.componentsHealthy,
		e.componentsDegraded,
		e.componentsCritical,
	)

	return e
}

// Integration interface implementation.

func (e *Exporter) Name() string { return "prometheus" }

func (e *Exporter) Initialize(_ context.Context) error {
	mux := http.NewServeMux()
	mux.Handle(e.config.MetricsPath, promhttp.HandlerFor(e.registry, promhttp.HandlerOpts{}))

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
		Message:   fmt.Sprintf("serving metrics on :%d%s", e.config.Port, e.config.MetricsPath),
		LastCheck: time.Now(),
	}
}

// MetricsRecorder interface implementation.

func (e *Exporter) RecordEvent(source string) {
	e.eventsTotal.WithLabelValues(source).Inc()
}

func (e *Exporter) RecordCollectorError(source string) {
	e.collectorErrors.WithLabelValues(source).Inc()
}

func (e *Exporter) RecordAnomaly() {
	e.anomaliesDetected.Inc()
}

func (e *Exporter) RecordIncidentCreated() {
	e.incidentsCreated.Inc()
}

func (e *Exporter) RecordIncidentResolved() {
	e.incidentsResolved.Inc()
}

func (e *Exporter) RecordRemediation(success bool) {
	e.remediationsTotal.Inc()
	if success {
		e.remediationsSuccess.Inc()
	} else {
		e.remediationsFailed.Inc()
	}
}

func (e *Exporter) SetMTTR(seconds float64) {
	e.mttrSeconds.Set(seconds)
}

func (e *Exporter) RecordPolicyEvaluation() {
	e.policyEvaluations.Inc()
}

func (e *Exporter) RecordPolicyViolation() {
	e.policyViolations.Inc()
}

func (e *Exporter) SetComponentHealth(healthy, degraded, critical int) {
	e.componentsHealthy.Set(float64(healthy))
	e.componentsDegraded.Set(float64(degraded))
	e.componentsCritical.Set(float64(critical))
}

func (e *Exporter) SetUptime(seconds float64) {
	e.uptimeSeconds.Set(seconds)
}
