package otel

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync/atomic"
	"time"

	"github.com/kronveil/kronveil/core/engine"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds OpenTelemetry exporter configuration.
type Config struct {
	Endpoint       string
	Insecure       bool
	ExportInterval time.Duration
}

// Exporter sends Kronveil metrics via OTLP gRPC.
// It implements both engine.Integration and engine.MetricsRecorder.
type Exporter struct {
	config        Config
	provider      *sdkmetric.MeterProvider
	conn          *grpc.ClientConn
	healthy       atomic.Bool

	// Counters
	eventsTotal        otelmetric.Int64Counter
	collectorErrors    otelmetric.Int64Counter
	anomaliesDetected  otelmetric.Int64Counter
	incidentsCreated   otelmetric.Int64Counter
	incidentsResolved  otelmetric.Int64Counter
	remediationsTotal  otelmetric.Int64Counter
	remediationsSuccess otelmetric.Int64Counter
	remediationsFailed otelmetric.Int64Counter
	policyEvaluations  otelmetric.Int64Counter
	policyViolations   otelmetric.Int64Counter

	// Gauge backing values (read by observable gauges)
	mttrVal            atomic.Int64 // float64 bits stored as int64
	uptimeVal          atomic.Int64
	healthyVal         atomic.Int64
	degradedVal        atomic.Int64
	criticalVal        atomic.Int64
}

// NewExporter creates a new OpenTelemetry OTLP gRPC exporter.
func NewExporter(config Config) *Exporter {
	return &Exporter{config: config}
}

// Integration interface implementation.

func (e *Exporter) Name() string { return "opentelemetry" }

func (e *Exporter) Initialize(ctx context.Context) error {
	// Establish gRPC connection.
	dialOpts := []grpc.DialOption{}
	if e.config.Insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(e.config.Endpoint, dialOpts...)
	if err != nil {
		return fmt.Errorf("failed to create gRPC connection: %w", err)
	}
	e.conn = conn

	// Create OTLP metric exporter.
	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create MeterProvider with periodic reader.
	interval := e.config.ExportInterval
	if interval == 0 {
		interval = 30 * time.Second
	}
	e.provider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(interval),
		)),
	)

	// Create meter and instruments.
	meter := e.provider.Meter("kronveil")

	e.eventsTotal, _ = meter.Int64Counter("kronveil.events.total",
		otelmetric.WithDescription("Total number of telemetry events received"))
	e.collectorErrors, _ = meter.Int64Counter("kronveil.collector.errors.total",
		otelmetric.WithDescription("Total number of collector errors"))
	e.anomaliesDetected, _ = meter.Int64Counter("kronveil.anomalies.detected",
		otelmetric.WithDescription("Total number of anomalies detected"))
	e.incidentsCreated, _ = meter.Int64Counter("kronveil.incidents.created",
		otelmetric.WithDescription("Total number of incidents created"))
	e.incidentsResolved, _ = meter.Int64Counter("kronveil.incidents.resolved",
		otelmetric.WithDescription("Total number of incidents resolved"))
	e.remediationsTotal, _ = meter.Int64Counter("kronveil.remediations.total",
		otelmetric.WithDescription("Total number of remediation actions"))
	e.remediationsSuccess, _ = meter.Int64Counter("kronveil.remediations.success",
		otelmetric.WithDescription("Total number of successful remediations"))
	e.remediationsFailed, _ = meter.Int64Counter("kronveil.remediations.failed",
		otelmetric.WithDescription("Total number of failed remediations"))
	e.policyEvaluations, _ = meter.Int64Counter("kronveil.policy.evaluations",
		otelmetric.WithDescription("Total number of policy evaluations"))
	e.policyViolations, _ = meter.Int64Counter("kronveil.policy.violations",
		otelmetric.WithDescription("Total number of policy violations"))

	// Register observable gauges with callbacks.
	if _, err := meter.Float64ObservableGauge("kronveil.mttr.seconds",
		otelmetric.WithDescription("Mean time to recovery in seconds"),
		otelmetric.WithFloat64Callback(func(_ context.Context, o otelmetric.Float64Observer) error {
			o.Observe(float64FromBits(e.mttrVal.Load()))
			return nil
		}),
	); err != nil {
		return fmt.Errorf("failed to create mttr gauge: %w", err)
	}
	if _, err := meter.Float64ObservableGauge("kronveil.uptime.seconds",
		otelmetric.WithDescription("Agent uptime in seconds"),
		otelmetric.WithFloat64Callback(func(_ context.Context, o otelmetric.Float64Observer) error {
			o.Observe(float64FromBits(e.uptimeVal.Load()))
			return nil
		}),
	); err != nil {
		return fmt.Errorf("failed to create uptime gauge: %w", err)
	}
	if _, err := meter.Int64ObservableGauge("kronveil.components.healthy",
		otelmetric.WithDescription("Number of healthy components"),
		otelmetric.WithInt64Callback(func(_ context.Context, o otelmetric.Int64Observer) error {
			o.Observe(e.healthyVal.Load())
			return nil
		}),
	); err != nil {
		return fmt.Errorf("failed to create healthy components gauge: %w", err)
	}
	if _, err := meter.Int64ObservableGauge("kronveil.components.degraded",
		otelmetric.WithDescription("Number of degraded components"),
		otelmetric.WithInt64Callback(func(_ context.Context, o otelmetric.Int64Observer) error {
			o.Observe(e.degradedVal.Load())
			return nil
		}),
	); err != nil {
		return fmt.Errorf("failed to create degraded components gauge: %w", err)
	}
	if _, err := meter.Int64ObservableGauge("kronveil.components.critical",
		otelmetric.WithDescription("Number of critical components"),
		otelmetric.WithInt64Callback(func(_ context.Context, o otelmetric.Int64Observer) error {
			o.Observe(e.criticalVal.Load())
			return nil
		}),
	); err != nil {
		return fmt.Errorf("failed to create critical components gauge: %w", err)
	}

	e.healthy.Store(true)
	log.Printf("[otel] OpenTelemetry exporter initialized (endpoint: %s, interval: %s)",
		e.config.Endpoint, interval)
	return nil
}

func (e *Exporter) Close() error {
	e.healthy.Store(false)
	if e.provider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e.provider.Shutdown(ctx); err != nil {
			log.Printf("[otel] Error shutting down meter provider: %v", err)
		}
	}
	if e.conn != nil {
		return e.conn.Close()
	}
	return nil
}

func (e *Exporter) Health() engine.ComponentHealth {
	status := "healthy"
	msg := fmt.Sprintf("exporting to %s", e.config.Endpoint)
	if !e.healthy.Load() {
		status = "degraded"
		msg = "exporter not initialized"
	}
	return engine.ComponentHealth{
		Name:      "opentelemetry-exporter",
		Status:    status,
		Message:   msg,
		LastCheck: time.Now(),
	}
}

// MetricsRecorder interface implementation.

func (e *Exporter) RecordEvent(source string) {
	e.eventsTotal.Add(context.Background(), 1, otelmetric.WithAttributes(
		attribute.String("source", source),
	))
}

func (e *Exporter) RecordCollectorError(source string) {
	e.collectorErrors.Add(context.Background(), 1, otelmetric.WithAttributes(
		attribute.String("source", source),
	))
}

func (e *Exporter) RecordAnomaly() {
	e.anomaliesDetected.Add(context.Background(), 1)
}

func (e *Exporter) RecordIncidentCreated() {
	e.incidentsCreated.Add(context.Background(), 1)
}

func (e *Exporter) RecordIncidentResolved() {
	e.incidentsResolved.Add(context.Background(), 1)
}

func (e *Exporter) RecordRemediation(success bool) {
	e.remediationsTotal.Add(context.Background(), 1)
	if success {
		e.remediationsSuccess.Add(context.Background(), 1)
	} else {
		e.remediationsFailed.Add(context.Background(), 1)
	}
}

func (e *Exporter) SetMTTR(seconds float64) {
	e.mttrVal.Store(float64ToBits(seconds))
}

func (e *Exporter) RecordPolicyEvaluation() {
	e.policyEvaluations.Add(context.Background(), 1)
}

func (e *Exporter) RecordPolicyViolation() {
	e.policyViolations.Add(context.Background(), 1)
}

func (e *Exporter) SetComponentHealth(healthy, degraded, critical int) {
	e.healthyVal.Store(int64(healthy))
	e.degradedVal.Store(int64(degraded))
	e.criticalVal.Store(int64(critical))
}

func (e *Exporter) SetUptime(seconds float64) {
	e.uptimeVal.Store(float64ToBits(seconds))
}

// float64ToBits converts a float64 to its int64 bit representation for atomic storage.
func float64ToBits(f float64) int64 {
	return int64(math.Float64bits(f))
}

// float64FromBits converts an int64 bit representation back to float64.
func float64FromBits(bits int64) float64 {
	return math.Float64frombits(uint64(bits))
}
