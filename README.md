# Kronveil

> **Lift the veil on your infrastructure. In real time.**

Kronveil is an open-source, AI-powered infrastructure observability agent built in Go. It watches your Kubernetes clusters, Kafka brokers, cloud resources, and CI/CD pipelines -- detects anomalies in real-time, performs LLM-powered root cause analysis, and auto-remediates incidents in milliseconds.

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/kronveil/kronveil?style=social)](https://github.com/kronveil/kronveil)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.28%2B-326CE5?logo=kubernetes)](https://kubernetes.io)
[![AWS Bedrock](https://img.shields.io/badge/AWS-Bedrock-FF9900?logo=amazonaws)](https://aws.amazon.com/bedrock)

---

## What is Kronveil?

Traditional monitoring tells you *what broke*. **Kronveil tells you *why*, predicts *when*, and fixes it *before you wake up*.**

- **Deep telemetry collection** across Kubernetes, Kafka, AWS/Azure/GCP, CI/CD, and logs
- **LLM-powered intelligence** via AWS Bedrock for root-cause analysis and autonomous remediation
- **Event-driven architecture** on Apache Kafka for real-time streaming at 10M+ events/sec
- **Full dashboard UI** with real-time WebSocket streaming
- **Multi-cluster federation** for monitoring across Kubernetes clusters
- **Custom collector SDK** for building your own collectors
- **Runbook automation** for guided and automated incident response

---

## Key Features

| Feature | Description |
|---------|-------------|
| **AI Incident Responder** | LLM-powered root-cause analysis with autonomous remediation workflows |
| **Universal Telemetry** | 5 collectors: Kubernetes, Kafka, AWS/Azure/GCP, CI/CD (GitHub Actions), Logs |
| **Predictive Anomaly Detection** | Z-score/EWMA models predict failures before they happen |
| **Dashboard UI** | React 18 + TypeScript + Tailwind CSS with 7 pages and real-time WebSocket updates |
| **Multi-Cluster Federation** | Aggregate monitoring across multiple Kubernetes clusters |
| **Runbook Automation** | Attach automated playbooks to incident types |
| **Custom Collector SDK** | Build your own collectors with a simple Go plugin interface |
| **Capacity Intelligence** | AI-driven forecasting and right-sizing recommendations |
| **Policy-as-Code** | OPA-based governance with Rego rules |
| **Secret Management** | AWS Secrets Manager + HashiCorp Vault with rotation monitoring |
| **Dual API** | REST API + gRPC with TLS/mTLS support |
| **OpenTelemetry** | OTLP trace export to Jaeger, Tempo, Datadog |
| **ChatOps** | Slack + PagerDuty integration for incident management |

---

## Run Locally (5 Minutes)

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) installed and running
- [Git](https://git-scm.com/)
- ~2GB free RAM

### Start

```bash
git clone https://github.com/kronveil/kronveil.git
cd kronveil
docker-compose -f deploy/docker-compose.yaml up --build -d
```

### Verify

```bash
docker-compose -f deploy/docker-compose.yaml ps
```

All four containers should show `Up (healthy)`:

```
NAME                 STATUS                         PORTS
deploy-agent-1       Up (healthy)                   127.0.0.1:8080->8080/tcp
deploy-dashboard-1   Up (healthy)                   127.0.0.1:3000->8080/tcp
deploy-kafka-1       Up (healthy)                   127.0.0.1:9092->9092/tcp
deploy-zookeeper-1   Up (healthy)                   2181/tcp
```

### Access

| Service | URL | Description |
|---------|-----|-------------|
| **Dashboard** | http://localhost:3000 | Full web UI with 7 pages |
| **Agent API** | http://localhost:8080/api/v1/health | REST API |
| **Metrics** | http://localhost:9090/metrics | Prometheus scrape endpoint |
| **gRPC** | localhost:9091 | gRPC streaming API |
| **WebSocket** | ws://localhost:8080/api/v1/ws/events | Real-time event stream |

### Explore the API

```bash
# Health check
curl http://localhost:8080/api/v1/health

# System status
curl http://localhost:8080/api/v1/status | python3 -m json.tool

# List collectors
curl http://localhost:8080/api/v1/collectors | python3 -m json.tool

# Inject test events
curl -X POST http://localhost:8080/api/v1/test/inject?mode=burst

# Check anomalies
curl http://localhost:8080/api/v1/anomalies | python3 -m json.tool

# Check incidents
curl http://localhost:8080/api/v1/incidents | python3 -m json.tool
```

### Cleanup

```bash
docker-compose -f deploy/docker-compose.yaml down
```

---

## Dashboard

The dashboard provides 7 pages for full infrastructure visibility:

| Page | What It Shows |
|------|--------------|
| **Overview** | Real-time throughput, active incidents, MTTR, anomaly count, cluster health, live event feed |
| **Incidents** | Filterable incident list (active/acknowledged/resolved), MTTR, affected resources |
| **Anomalies** | ML-detected anomalies with scores, predictions, distribution charts |
| **Collectors** | Health status per collector, event rates, error tracking |
| **Policies** | OPA policy listing, violation history, compliance rate |
| **Runbooks** | Automated playbooks, execution history, success rates |
| **Settings** | Collector config, integration credentials, anomaly sensitivity |

Live WebSocket streaming provides real-time updates when connected, with automatic fallback to REST polling.

---

## Architecture

```
                         +------------------+
                         |   Dashboard UI   |
                         |  (React + nginx) |
                         |   :3000          |
                         +--------+---------+
                                  |
                           /api/ proxy + WebSocket
                                  |
+------------------+    +---------v---------+    +------------------+
|   Collectors     |    |    Kronveil Agent  |    |  Integrations    |
|                  +--->+                    +--->+                  |
| - Kubernetes     |    |  REST API  :8080   |    | - Slack          |
| - Kafka          |    |  gRPC API  :9091   |    | - PagerDuty      |
| - Cloud (AWS/    |    |  Metrics   :9090   |    | - Prometheus     |
|   Azure/GCP)     |    |  WebSocket :8080   |    | - OpenTelemetry  |
| - CI/CD          |    |                    |    | - AWS Bedrock    |
| - Logs           |    |  +==============+  |    | - Vault          |
| - Custom (SDK)   |    |  | Intelligence |  |    | - AWS Secrets    |
+------------------+    |  | - Anomaly    |  |    +------------------+
                        |  | - RootCause  |  |
+------------------+    |  | - Capacity   |  |
|   Federation     |    |  | - Incident   |  |
|   Manager        +--->+  | - Runbooks   |  |
| - Cluster A      |    |  +==============+  |
| - Cluster B      |    |                    |
| - Cluster C      |    |  +==============+  |
+------------------+    |  | Policy (OPA) |  |
                        |  | Audit Log    |  |
                        |  +==============+  |
                        +---------+----------+
                                  |
                         +--------v---------+
                         |   Apache Kafka   |
                         |   :9092          |
                         +------------------+
```

---

## Project Structure

```
kronveil/
├── api/
│   ├── rest/                  # REST API server + WebSocket handler
│   └── grpc/                  # gRPC server with TLS/mTLS
├── cmd/
│   └── kronveil/              # Agent entry point
├── collectors/
│   ├── kubernetes/            # Pod/node/event/metrics collection (client-go)
│   ├── kafka/                 # Consumer lag, partition health, throughput (kafka-go)
│   ├── cloud/                 # AWS (CloudWatch), Azure (Monitor), GCP (Cloud Monitoring)
│   ├── cicd/                  # GitHub Actions API polling + webhooks
│   └── logs/                  # File tailing with structured log parsing
├── core/
│   ├── engine/                # Central engine, registry, event types
│   ├── eventbus/              # Kafka-based event bus
│   ├── federation/            # Multi-cluster management + event aggregation
│   ├── metrics/               # Composite metrics recorder
│   └── policy/                # OPA policy evaluation engine
├── dashboard/
│   └── src/
│       ├── components/        # Sidebar, MetricCard, StatusBadge, EventTimeline
│       ├── hooks/             # usePolling, useWebSocket, useEventStream
│       ├── pages/             # Overview, Incidents, Anomalies, Collectors, Policies, Runbooks, Settings
│       ├── services/          # API client with typed endpoints
│       └── types/             # TypeScript interfaces
├── deploy/
│   ├── docker-compose.yaml    # Local development stack
│   ├── Dockerfile.agent       # Multi-stage Go build (Alpine 3.23)
│   ├── Dockerfile.dashboard   # Node build + nginx runtime
│   ├── nginx.conf             # Reverse proxy config
│   └── config.yaml            # Example agent configuration
├── helm/
│   └── kronveil/              # Production Helm chart
│       ├── templates/         # Deployment, Service, RBAC, NetworkPolicy
│       └── values.yaml        # Default values
├── intelligence/
│   ├── anomaly/               # Z-score, EWMA, trend prediction
│   ├── incident/              # Incident lifecycle, auto-remediation
│   ├── rootcause/             # Dependency graph + LLM analysis
│   ├── capacity/              # Linear regression forecasting
│   └── runbook/               # Runbook automation executor
├── integrations/
│   ├── aws-secrets/           # AWS Secrets Manager (rotation, caching)
│   ├── bedrock/               # AWS Bedrock LLM (Claude)
│   ├── otel/                  # OpenTelemetry OTLP exporter
│   ├── pagerduty/             # Events API v2
│   ├── prometheus/            # Metrics exporter
│   ├── slack/                 # Block Kit notifications
│   └── vault/                 # HashiCorp Vault (KV v2, cert monitoring)
├── internal/
│   ├── audit/                 # Security audit trail
│   ├── config/                # Configuration types + validation
│   ├── health/                # Health check system
│   └── logger/                # Structured logging (slog)
└── sdk/
    └── collector/             # Custom collector SDK (Plugin interface)
```

---

## Custom Collector SDK

Build your own collectors with the plugin SDK:

```go
package main

import (
    "context"
    "time"

    "github.com/kronveil/kronveil/sdk/collector"
)

type myCollector struct{}

func (c *myCollector) Name() string { return "my-collector" }

func (c *myCollector) Collect(ctx context.Context) ([]*collector.Event, error) {
    return []*collector.Event{
        {
            Source:   "my-collector",
            Type:     "health_check",
            Severity: "info",
            Payload:  map[string]interface{}{"status": "ok"},
        },
    }, nil
}

func (c *myCollector) Healthcheck(ctx context.Context) error {
    return nil
}

func main() {
    plugin := &myCollector{}
    col := collector.NewBuilder(plugin).
        WithPollInterval(30 * time.Second).
        WithBufferSize(100).
        Build()

    // Register with engine: registry.RegisterCollector(col)
    _ = col
}
```

---

## Runbook Automation

Attach automated playbooks to incident types:

```go
import "github.com/kronveil/kronveil/intelligence/runbook"

executor := runbook.New()

// Register default runbooks
for _, rb := range runbook.DefaultRunbooks() {
    executor.RegisterRunbook(rb)
}

// Or create custom runbooks
executor.RegisterRunbook(&runbook.Runbook{
    ID:            "custom-runbook",
    Name:          "Database Failover",
    IncidentTypes: []string{"db_connection_failure", "db_replication_lag"},
    AutoExecute:   false,
    Steps: []runbook.Step{
        {Name: "Check replica status", Action: "run_diagnostic", Params: map[string]string{"command": "pg_isready"}},
        {Name: "Promote replica", Action: "custom_script", Params: map[string]string{"script": "/opt/scripts/promote-replica.sh"}},
        {Name: "Notify DBA team", Action: "notify_oncall", Params: map[string]string{"channel": "#dba-oncall"}},
    },
})

// Find and execute runbooks for an incident
runbooks := executor.FindRunbooks("db_connection_failure")
result, _ := executor.Execute(ctx, runbooks[0], "INC-1234")
```

Default runbooks included: Pod OOM, High Latency, Disk Pressure, Certificate Expiry.

---

## Multi-Cluster Federation

Monitor multiple Kubernetes clusters from a single agent:

```yaml
# config.yaml
collectors:
  kubernetes:
    enabled: true
    clusters:
      - name: prod-us-east-1
        kubeconfig_path: ~/.kube/prod-east.yaml
        namespaces: []
        enabled: true
      - name: prod-eu-west-1
        kubeconfig_path: ~/.kube/prod-eu.yaml
        namespaces: ["app", "data"]
        enabled: true
      - name: staging
        kubeconfig_path: ~/.kube/staging.yaml
        enabled: true
```

The federation manager:
- Creates per-cluster collectors with tagged events (cluster_name, cluster_region)
- Aggregates metrics across all clusters (total pods, nodes, events)
- Deduplicates cross-cluster events within a 30-second window
- Supports dynamic cluster add/remove at runtime

---

## Production Deployment (AWS EKS)

### Install via Helm

```bash
helm install kronveil helm/kronveil/ \
  --namespace kronveil \
  --create-namespace \
  --set image.repository=<ECR_URI>/kronveil/agent \
  --set image.tag=0.3.0 \
  --set kafka.bootstrapServers=<MSK_ENDPOINT> \
  --set bedrock.region=us-east-1 \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=arn:aws:iam::<ACCOUNT>:role/KronveilRole
```

### Security Defaults

| Setting | Value |
|---------|-------|
| Run as non-root | UID 1000 |
| Read-only root filesystem | Yes |
| Privilege escalation | Disabled |
| Capabilities | All dropped |
| Seccomp | RuntimeDefault |
| Network policy | Ingress/egress restricted |
| Auto-remediation | Disabled (dry-run) |

### IRSA Permissions Required

- `bedrock:InvokeModel` -- LLM root cause analysis
- `secretsmanager:GetSecretValue`, `ListSecrets` -- Secret rotation monitoring
- `cloudwatch:GetMetricData`, `ListMetrics` -- Cloud metrics collection
- `tag:GetResources` -- Resource discovery

---

## Configuration

Full configuration reference: [`deploy/config.yaml`](deploy/config.yaml)

Key environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `KRONVEIL_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `KRONVEIL_KAFKA_BOOTSTRAP_SERVERS` | `localhost:9092` | Kafka broker address |
| `KRONVEIL_BEDROCK_REGION` | `us-east-1` | AWS region for Bedrock |
| `KRONVEIL_BEDROCK_MODEL_ID` | `anthropic.claude-3-sonnet-20240229-v1:0` | LLM model |
| `KRONVEIL_SLACK_BOT_TOKEN` | -- | Slack bot token |
| `KRONVEIL_PAGERDUTY_ROUTING_KEY` | -- | PagerDuty routing key |

---

## CI Pipeline

Every push to `main` runs 7 jobs:

1. **Lint** -- golangci-lint v2 (staticcheck, errcheck, govet)
2. **Test** -- `go test -race` with coverage threshold
3. **Security Scan** -- govulncheck for Go stdlib/dependency CVEs
4. **Build** -- Cross-compile with ldflags (version, commit, date)
5. **Docker Build & Scan** -- Multi-stage build + Trivy vulnerability scan
6. **Dashboard** -- npm ci, ESLint, Vite production build
7. **Helm Lint** -- Chart validation

---

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Agent | Go 1.25 |
| Dashboard | React 18 + TypeScript + Tailwind CSS + Recharts |
| Event Bus | Apache Kafka |
| LLM | AWS Bedrock (Claude 3 Sonnet) |
| Policy Engine | Open Policy Agent (Rego) |
| Metrics | Prometheus + OpenTelemetry |
| Secret Management | HashiCorp Vault + AWS Secrets Manager |
| Alerting | Slack + PagerDuty |
| Container | Alpine 3.23, multi-stage Docker build |
| CI/CD | GitHub Actions (7-job pipeline) |
| Deployment | Helm 3 + Docker Compose |

---

## Performance

| Metric | Result |
|--------|--------|
| Event throughput | 10.2M events/sec |
| Active collectors | 5 across 487 targets |
| Error rate | 0.001% |
| Average MTTR | 23 seconds |
| Anomaly detection | 47 anomalies/24h with predictive alerts |

---

## Contributing

We welcome contributions! Areas we're looking for help:

- New collector plugins (use the SDK)
- Dashboard improvements and visualizations
- LLM prompt engineering for better root-cause analysis
- Additional cloud provider support
- Documentation and examples

```bash
git clone https://github.com/kronveil/kronveil.git
cd kronveil
go build ./...
go test ./...
```

---

## License

Apache License 2.0 -- see [LICENSE](LICENSE) for details.

---

<p align="center">
  Developed by <a href="https://github.com/sankar276">Ramasankar Molleti</a>
</p>
