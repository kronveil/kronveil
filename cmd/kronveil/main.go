package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcserver "github.com/kronveil/kronveil/api/grpc"
	"github.com/kronveil/kronveil/api/rest"
	cloudcollector "github.com/kronveil/kronveil/collectors/cloud"
	k8scollector "github.com/kronveil/kronveil/collectors/kubernetes"
	kafkacollector "github.com/kronveil/kronveil/collectors/kafka"
	"github.com/kronveil/kronveil/core/engine"
	"github.com/kronveil/kronveil/core/eventbus"
	"github.com/kronveil/kronveil/core/metrics"
	"github.com/kronveil/kronveil/intelligence/anomaly"
	"github.com/kronveil/kronveil/intelligence/capacity"
	"github.com/kronveil/kronveil/intelligence/incident"
	"github.com/kronveil/kronveil/intelligence/rootcause"
	"github.com/kronveil/kronveil/internal/config"
	"github.com/kronveil/kronveil/internal/version"
	awssecrets "github.com/kronveil/kronveil/integrations/aws-secrets"
	"github.com/kronveil/kronveil/integrations/bedrock"
	otelexporter "github.com/kronveil/kronveil/integrations/otel"
	promexporter "github.com/kronveil/kronveil/integrations/prometheus"
	slacknotifier "github.com/kronveil/kronveil/integrations/slack"
	vaultclient "github.com/kronveil/kronveil/integrations/vault"
	logscollector "github.com/kronveil/kronveil/collectors/logs"
	pdclient "github.com/kronveil/kronveil/integrations/pagerduty"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version.Info())
		return
	}

	log.Println("=== Kronveil Agent ===")
	log.Println(version.Info())
	log.Println()

	// Load configuration.
	cfg := config.DefaultConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize structured logging.
	var logHandler slog.Handler
	if cfg.Agent.LogFormat == "json" {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: parseSlogLevel(cfg.Agent.LogLevel),
		})
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: parseSlogLevel(cfg.Agent.LogLevel),
		})
	}
	slog.SetDefault(slog.New(logHandler))

	// Create event bus.
	bus, err := eventbus.NewKafkaEventBus(eventbus.KafkaConfig{
		BootstrapServers: cfg.Kafka.BootstrapServers,
		GroupID:          cfg.Kafka.GroupID,
	})
	if err != nil {
		log.Fatalf("Failed to create event bus: %v", err)
	}

	// Create component registry.
	registry := engine.NewRegistry()

	// Set up metrics backends.
	var recorders []engine.MetricsRecorder

	if cfg.Integrations.Prometheus.Enabled {
		prom := promexporter.NewExporter(promexporter.Config{
			Port:        cfg.Integrations.Prometheus.Port,
			MetricsPath: cfg.Integrations.Prometheus.MetricsPath,
		})
		if err := registry.RegisterIntegration(prom); err != nil {
			log.Fatalf("Failed to register prometheus exporter: %v", err)
		}
		recorders = append(recorders, prom)
		log.Printf("  Prometheus: http://localhost:%d%s",
			cfg.Integrations.Prometheus.Port, cfg.Integrations.Prometheus.MetricsPath)
	}

	if cfg.Integrations.OpenTelemetry.Enabled {
		exportInterval, _ := time.ParseDuration(cfg.Integrations.OpenTelemetry.ExportInterval)
		if exportInterval == 0 {
			exportInterval = 30 * time.Second
		}
		otel := otelexporter.NewExporter(otelexporter.Config{
			Endpoint:       cfg.Integrations.OpenTelemetry.Endpoint,
			Insecure:       cfg.Integrations.OpenTelemetry.Insecure,
			ExportInterval: exportInterval,
		})
		if err := registry.RegisterIntegration(otel); err != nil {
			log.Fatalf("Failed to register otel exporter: %v", err)
		}
		recorders = append(recorders, otel)
		log.Printf("  OpenTelemetry: %s", cfg.Integrations.OpenTelemetry.Endpoint)
	}

	metricsRecorder := metrics.NewCompositeRecorder(recorders...)

	// Register Vault integration.
	if cfg.Integrations.Vault.Enabled {
		vc, err := vaultclient.NewClient(vaultclient.Config{
			Address:    cfg.Integrations.Vault.Address,
			AuthMethod: cfg.Integrations.Vault.AuthMethod,
		})
		if err != nil {
			log.Printf("WARNING: Failed to create Vault client: %v", err)
		} else {
			if err := registry.RegisterIntegration(vc); err != nil {
				log.Fatalf("Failed to register Vault integration: %v", err)
			}
			log.Printf("  Vault: %s", cfg.Integrations.Vault.Address)
		}
	}

	// Register AWS Secrets Manager integration.
	if cfg.Integrations.AWSSecrets.Enabled {
		rotationWindow, _ := time.ParseDuration(cfg.Integrations.AWSSecrets.RotationWindow)
		asc, err := awssecrets.NewClient(awssecrets.Config{
			Region:         cfg.Integrations.AWSSecrets.Region,
			SecretPrefix:   cfg.Integrations.AWSSecrets.SecretPrefix,
			RotationWindow: rotationWindow,
			CacheEnabled:   cfg.Integrations.AWSSecrets.CacheEnabled,
		})
		if err != nil {
			log.Printf("WARNING: Failed to create AWS Secrets Manager client: %v", err)
		} else {
			if err := registry.RegisterIntegration(asc); err != nil {
				log.Fatalf("Failed to register AWS Secrets Manager: %v", err)
			}
			log.Printf("  AWS Secrets Manager: %s", cfg.Integrations.AWSSecrets.Region)
		}
	}

	// Register collectors.
	if cfg.Collectors.Kubernetes.Enabled {
		k8s := k8scollector.New(k8scollector.Config{
			Kubeconfig:   cfg.Collectors.Kubernetes.Kubeconfig,
			Namespaces:   cfg.Collectors.Kubernetes.Namespaces,
			PollInterval: cfg.Collectors.Kubernetes.PollInterval,
		})
		if err := registry.RegisterCollector(k8s); err != nil {
			log.Fatalf("Failed to register k8s collector: %v", err)
		}
	}

	if cfg.Collectors.Kafka.Enabled {
		kc := kafkacollector.New(kafkacollector.Config{
			BootstrapServers: cfg.Collectors.Kafka.BootstrapServers,
			MonitoredTopics:  cfg.Collectors.Kafka.MonitoredTopics,
			ConsumerGroups:   cfg.Collectors.Kafka.ConsumerGroups,
			LagThreshold:     cfg.Collectors.Kafka.LagThreshold,
			PollInterval:     cfg.Collectors.Kafka.PollInterval,
		})
		if err := registry.RegisterCollector(kc); err != nil {
			log.Fatalf("Failed to register kafka collector: %v", err)
		}
	}

	// Register cloud collector.
	if cfg.Collectors.Cloud.Enabled {
		cc, err := cloudcollector.New(cloudcollector.Config{
			Provider: cloudcollector.ProviderType(cfg.Collectors.Cloud.Provider),
			Regions:  cfg.Collectors.Cloud.Regions,
		})
		if err != nil {
			log.Printf("WARNING: Failed to create cloud collector: %v", err)
		} else {
			if err := registry.RegisterCollector(cc); err != nil {
				log.Fatalf("Failed to register cloud collector: %v", err)
			}
		}
	}

	// Create Bedrock LLM provider.
	var llmProvider engine.LLMProvider
	brClient, err := bedrock.NewClient(bedrock.Config{
		Region:      cfg.Bedrock.Region,
		ModelID:     cfg.Bedrock.ModelID,
		MaxTokens:   cfg.Bedrock.MaxTokens,
		Temperature: cfg.Bedrock.Temperature,
	})
	if err != nil {
		log.Printf("WARNING: Failed to create Bedrock client: %v", err)
	} else {
		llmProvider = brClient
	}

	// Create notifiers (Slack + PagerDuty).
	var notifiers []engine.Notifier
	if cfg.Integrations.Slack.Enabled && cfg.Integrations.Slack.BotToken != "" {
		sn, err := slacknotifier.NewNotifier(slacknotifier.Config{
			BotToken:       cfg.Integrations.Slack.BotToken,
			DefaultChannel: cfg.Integrations.Slack.DefaultChannel,
			Channels:       cfg.Integrations.Slack.Channels,
		})
		if err != nil {
			log.Printf("WARNING: Failed to create Slack notifier: %v", err)
		} else {
			notifiers = append(notifiers, sn)
			log.Printf("  Slack: %s", cfg.Integrations.Slack.DefaultChannel)
		}
	}

	if cfg.Integrations.PagerDuty.Enabled && cfg.Integrations.PagerDuty.RoutingKey != "" {
		pd, err := pdclient.NewClient(pdclient.Config{
			RoutingKey: cfg.Integrations.PagerDuty.RoutingKey,
		})
		if err != nil {
			log.Printf("WARNING: Failed to create PagerDuty client: %v", err)
		} else {
			if err := registry.RegisterIntegration(pd); err != nil {
				log.Fatalf("Failed to register PagerDuty integration: %v", err)
			}
			notifiers = append(notifiers, pd)
			log.Println("  PagerDuty: enabled")
		}
	}

	// Register logs collector.
	if cfg.Collectors.Logs.Enabled {
		var sources []logscollector.LogSource
		lc := logscollector.New(logscollector.Config{
			Sources:       sources,
			ErrorPatterns: cfg.Collectors.Logs.ErrorPatterns,
			ParseFormat:   cfg.Collectors.Logs.ParseFormat,
		})
		if err := registry.RegisterCollector(lc); err != nil {
			log.Fatalf("Failed to register logs collector: %v", err)
		}
	}

	// Register intelligence modules.
	detector := anomaly.New(anomaly.Config{
		WindowSize:      cfg.Intelligence.AnomalyDetection.WindowSize,
		ZScoreThreshold: cfg.Intelligence.AnomalyDetection.ZScoreThreshold,
		Sensitivity:     cfg.Intelligence.AnomalyDetection.Sensitivity,
	})
	detector.SetMetrics(metricsRecorder)
	if err := registry.RegisterModule(detector); err != nil {
		log.Fatalf("Failed to register anomaly detector: %v", err)
	}

	responder := incident.New(incident.Config{
		AutoRemediate: cfg.Intelligence.Remediation.AutoRemediate,
		DryRun:        cfg.Intelligence.Remediation.DryRun,
		MaxRetries:    cfg.Intelligence.Remediation.MaxRetries,
	}, notifiers, llmProvider)
	responder.SetMetrics(metricsRecorder)
	if err := registry.RegisterModule(responder); err != nil {
		log.Fatalf("Failed to register incident responder: %v", err)
	}

	depGraph := rootcause.NewDependencyGraph()
	rcAnalyzer := rootcause.New(depGraph, llmProvider)
	if err := registry.RegisterModule(rcAnalyzer); err != nil {
		log.Fatalf("Failed to register root cause analyzer: %v", err)
	}

	capPlanner := capacity.New(capacity.Config{})
	if err := registry.RegisterModule(capPlanner); err != nil {
		log.Fatalf("Failed to register capacity planner: %v", err)
	}

	// Create and start the engine.
	eng := engine.NewEngine(registry, bus, metricsRecorder)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := eng.Start(ctx); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	// Start REST API server.
	apiServer := rest.NewServer(rest.Config{
		Port:        cfg.API.RESTPort,
		APIKey:      cfg.API.APIKey,
		TLSCertFile: cfg.API.TLS.CertFile,
		TLSKeyFile:  cfg.API.TLS.KeyFile,
		TLSCAFile:   cfg.API.TLS.CAFile,
		MutualTLS:   cfg.API.TLS.MutualTLS,
	}, eng, responder, detector)
	if err := apiServer.Start(); err != nil {
		log.Fatalf("Failed to start REST API: %v", err)
	}

	// Start gRPC server.
	grpcSrv := grpcserver.NewServer(grpcserver.Config{
		Port:        cfg.API.GRPCPort,
		TLSCertFile: cfg.API.TLS.CertFile,
		TLSKeyFile:  cfg.API.TLS.KeyFile,
		TLSCAFile:   cfg.API.TLS.CAFile,
		MutualTLS:   cfg.API.TLS.MutualTLS,
	}, eng, responder)
	if err := grpcSrv.Start(); err != nil {
		log.Fatalf("Failed to start gRPC server: %v", err)
	}

	log.Println()
	log.Println("Kronveil agent is running. Press Ctrl+C to stop.")
	log.Printf("  REST API:   http://localhost:%d", cfg.API.RESTPort)
	log.Printf("  gRPC API:   localhost:%d", cfg.API.GRPCPort)
	log.Printf("  Dashboard:  http://localhost:%d", cfg.API.RESTPort)

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println()
	log.Println("Shutting down Kronveil agent...")

	cancel()
	if err := grpcSrv.Stop(); err != nil {
		log.Printf("Error stopping gRPC server: %v", err)
	}
	if err := apiServer.Stop(); err != nil {
		log.Printf("Error stopping API server: %v", err)
	}
	if err := eng.Stop(); err != nil {
		log.Printf("Error stopping engine: %v", err)
	}
	if err := bus.Close(); err != nil {
		log.Printf("Error closing event bus: %v", err)
	}

	log.Println("Kronveil agent stopped. Goodbye.")
}

func parseSlogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
