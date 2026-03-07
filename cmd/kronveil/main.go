package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kronveil/kronveil/api/rest"
	"github.com/kronveil/kronveil/core/engine"
	"github.com/kronveil/kronveil/core/eventbus"
	"github.com/kronveil/kronveil/intelligence/anomaly"
	"github.com/kronveil/kronveil/intelligence/incident"
	"github.com/kronveil/kronveil/intelligence/rootcause"
	"github.com/kronveil/kronveil/intelligence/capacity"
	"github.com/kronveil/kronveil/internal/config"
	"github.com/kronveil/kronveil/internal/version"
	k8scollector "github.com/kronveil/kronveil/collectors/kubernetes"
	kafkacollector "github.com/kronveil/kronveil/collectors/kafka"
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

	// Register intelligence modules.
	detector := anomaly.New(anomaly.Config{
		WindowSize:      cfg.Intelligence.AnomalyDetection.WindowSize,
		ZScoreThreshold: cfg.Intelligence.AnomalyDetection.ZScoreThreshold,
		Sensitivity:     cfg.Intelligence.AnomalyDetection.Sensitivity,
	})
	if err := registry.RegisterModule(detector); err != nil {
		log.Fatalf("Failed to register anomaly detector: %v", err)
	}

	responder := incident.New(incident.Config{
		AutoRemediate: cfg.Intelligence.Remediation.AutoRemediate,
		DryRun:        cfg.Intelligence.Remediation.DryRun,
		MaxRetries:    cfg.Intelligence.Remediation.MaxRetries,
	}, nil, nil)
	if err := registry.RegisterModule(responder); err != nil {
		log.Fatalf("Failed to register incident responder: %v", err)
	}

	depGraph := rootcause.NewDependencyGraph()
	rcAnalyzer := rootcause.New(depGraph, nil)
	if err := registry.RegisterModule(rcAnalyzer); err != nil {
		log.Fatalf("Failed to register root cause analyzer: %v", err)
	}

	capPlanner := capacity.New(capacity.Config{})
	if err := registry.RegisterModule(capPlanner); err != nil {
		log.Fatalf("Failed to register capacity planner: %v", err)
	}

	// Create and start the engine.
	eng := engine.NewEngine(registry, bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := eng.Start(ctx); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	// Start REST API server.
	apiServer := rest.NewServer(rest.Config{
		Port:   cfg.API.RESTPort,
		APIKey: cfg.API.APIKey,
	}, eng, responder)
	if err := apiServer.Start(); err != nil {
		log.Fatalf("Failed to start REST API: %v", err)
	}

	log.Println()
	log.Println("Kronveil agent is running. Press Ctrl+C to stop.")
	log.Printf("  REST API:   http://localhost:%d", cfg.API.RESTPort)
	log.Printf("  Dashboard:  http://localhost:%d", cfg.API.RESTPort)

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println()
	log.Println("Shutting down Kronveil agent...")

	cancel()
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
