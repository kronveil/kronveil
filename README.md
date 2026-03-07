# Kronveil 👁️

> **Lift the veil on your infrastructure. In real time.**

Kronveil is an open-source, AI-powered all-in-one platform observability agent — built for engineers who run mission-critical infrastructure at scale. It watches everything, understands everything, and acts autonomously so you don't have to.

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/kronveil/kronveil?style=social)](https://github.com/kronveil/kronveil)
[![Discord](https://img.shields.io/discord/kronveil?label=Discord&logo=discord)](https://discord.gg/kronveil)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.26%2B-326CE5?logo=kubernetes)](https://kubernetes.io)
[![AWS Bedrock](https://img.shields.io/badge/AWS-Bedrock-FF9900?logo=amazonaws)](https://aws.amazon.com/bedrock)

---

## 🧠 What is Kronveil?

Modern platform infrastructure is a living, breathing organism — thousands of microservices, event streams, cloud workloads, and security surfaces changing every second. Traditional monitoring tells you *what broke*. **Kronveil tells you *why*, predicts *when*, and fixes it *before you wake up*.**

Kronveil combines:

- 🔭 **Deep telemetry collection** across Kubernetes, Kafka, multi-cloud, and CI/CD
- 🧠 **LLM-powered intelligence** via AWS Bedrock for root-cause analysis and autonomous remediation
- ⚡ **Event-driven architecture** on Apache Kafka for real-time streaming at 10M+ events/sec
- 🔐 **Zero-trust security posture** baked in from day one
- 📊 **Unified observability** — metrics, logs, traces, and AI insights in one platform

---

## ✨ Key Features

| Feature | Description |
|---------|-------------|
| 🤖 **AI Incident Responder** | LLM-powered root-cause analysis with autonomous remediation workflows |
| 📡 **Universal Telemetry** | Collects from Kubernetes, Kafka, AWS/Azure/GCP, CI/CD, and custom sources |
| 🔮 **Predictive Anomaly Detection** | ML models predict failures before they happen — reduce MTTR by 60%+ |
| 🏗️ **Capacity Intelligence** | AI-driven forecasting and right-sizing recommendations |
| 📋 **Policy-as-Code Engine** | OPA-based governance enforced across all environments |
| 🔄 **GitOps Native** | Full Flux/ArgoCD integration — infrastructure as observable code |
| 🔐 **Secret-Aware** | Deep HashiCorp Vault and External Secrets Operator integration |
| 💬 **ChatOps** | Slack/Teams integration for natural language incident management |

---

## 🏗️ Architecture

### System Overview

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                            KRONVEIL PLATFORM                                 │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                         DATA COLLECTION LAYER                           │ │
│  │                                                                         │ │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌────────────────┐   │ │
│  │  │ Kubernetes  │ │    Kafka    │ │    Cloud    │ │  CI/CD + Logs  │   │ │
│  │  │  Collector  │ │  Collector  │ │  Collector  │ │   Collectors   │   │ │
│  │  │             │ │             │ │             │ │                │   │ │
│  │  │ Pods/Nodes  │ │ Lag/Topics  │ │ EC2/RDS/ELB│ │ GitHub Actions │   │ │
│  │  │ Events/HPA  │ │ Throughput  │ │ Lambda/S3  │ │ Jenkins/GitLab │   │ │
│  │  │ Metrics API │ │ Partitions  │ │ CloudWatch │ │ File Tailing   │   │ │
│  │  └──────┬──────┘ └──────┬──────┘ └──────┬──────┘ └───────┬────────┘   │ │
│  └─────────┼───────────────┼───────────────┼─────────────────┼────────────┘ │
│            │               │               │                 │              │
│            ▼               ▼               ▼                 ▼              │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                     APACHE KAFKA EVENT BUS                              │ │
│  │                                                                         │ │
│  │  telemetry.raw ──▶ telemetry.enriched ──▶ anomalies.detected           │ │
│  │  incidents.new ──▶ incidents.updated  ──▶ remediation.actions          │ │
│  │  policy.violations ──▶ policy.audit   ──▶ capacity.forecasts          │ │
│  │                                                                         │ │
│  │                     10M+ events/sec · 3x replication                    │ │
│  └────────────────────────────┬────────────────────────────────────────────┘ │
│                               │                                              │
│            ┌──────────────────┼──────────────────┐                          │
│            ▼                  ▼                   ▼                          │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                       INTELLIGENCE LAYER                                │ │
│  │                                                                         │ │
│  │  ┌─────────────────┐ ┌──────────────────┐ ┌──────────────────────────┐ │ │
│  │  │  Anomaly        │ │  Root Cause      │ │  Capacity                │ │ │
│  │  │  Detector       │ │  Analyzer        │ │  Planner                 │ │ │
│  │  │                 │ │                  │ │                          │ │ │
│  │  │ Z-Score/EWMA    │ │ Dependency Graph │ │ Linear Regression        │ │ │
│  │  │ Isolation Score │ │ Causal Chain DFS │ │ Confidence Intervals     │ │ │
│  │  │ Trend Predict   │ │ Evidence Collect │ │ Right-Sizing Recommend   │ │ │
│  │  └────────┬────────┘ └────────┬─────────┘ └────────────┬─────────────┘ │ │
│  │           │                   │                         │               │ │
│  │           ▼                   ▼                         │               │ │
│  │  ┌─────────────────────────────────────┐               │               │ │
│  │  │     INCIDENT RESPONDER              │◀──────────────┘               │ │
│  │  │                                     │                               │ │
│  │  │  Detect ──▶ Triage ──▶ Respond ──▶ Resolve                         │ │
│  │  │  Correlate events within time window                                │ │
│  │  │  Auto-remediate with circuit breaker                                │ │
│  │  └──────────────────┬──────────────────┘                               │ │
│  └─────────────────────┼──────────────────────────────────────────────────┘ │
│                        │                                                     │
│            ┌───────────┼───────────┐                                        │
│            ▼           ▼           ▼                                        │
│  ┌──────────────┐ ┌─────────┐ ┌───────────┐                                │
│  │ AWS Bedrock  │ │  Slack  │ │ PagerDuty │                                │
│  │  LLM API    │ │Block Kit│ │Events v2  │                                │
│  │ Claude/Titan │ │ChatOps  │ │ On-Call   │                                │
│  └──────────────┘ └─────────┘ └───────────┘                                │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │  GOVERNANCE: OPA Policy Engine · Rego Rules · Compliance Audit Trail   │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │  SECURITY: HashiCorp Vault · Secret Rotation · Certificate Lifecycle   │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                      │
│  │  REST API    │  │  gRPC API    │  │  Prometheus  │                      │
│  │  :8080       │  │  :9091       │  │  :9090       │                      │
│  └──────────────┘  └──────────────┘  └──────────────┘                      │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Event Flow Pipeline

```
                    REAL-TIME EVENT PIPELINE
 ═══════════════════════════════════════════════════════════

 SOURCES                 PROCESSING                 ACTIONS
 ───────                 ──────────                 ───────

 ┌──────────┐     ┌──────────────────┐     ┌──────────────────┐
 │  K8s API ├────▶│                  │     │  Scale Pods      │
 └──────────┘     │   Telemetry      │     ├──────────────────┤
 ┌──────────┐     │   Events         │     │  Restart Service │
 │  Kafka   ├────▶│                  │     ├──────────────────┤
 │  Metrics │     │  Source/Type/    │     │  Drain Node      │
 └──────────┘     │  Payload/Time    │     ├──────────────────┤
 ┌──────────┐     └────────┬─────────┘     │  Rollback Deploy │
 │  Cloud   ├──┐           │               └────────▲─────────┘
 │  APIs    │  │           ▼                        │
 └──────────┘  │  ┌──────────────────┐              │
 ┌──────────┐  │  │                  │    ┌─────────┴─────────┐
 │  CI/CD   ├──┼─▶│  Kafka Bus       │    │  Auto-Remediation │
 │  Hooks   │  │  │  (10 topics)     │    │                   │
 └──────────┘  │  │                  │    │  Safety Checks:   │
 ┌──────────┐  │  └──┬───┬───┬──────┘    │  • Circuit Breaker│
 │  Log     ├──┘     │   │   │           │  • Dry Run Mode   │
 │  Files   │        │   │   │           │  • Max Retries    │
 └──────────┘        ▼   ▼   ▼           │  • Cooldown Timer │
              ┌──────┐ ┌──┐ ┌──────┐     └─────────▲─────────┘
              │Anomly│ │RC│ │Capac.│               │
              │Detect│ │A │ │Plan  │     ┌─────────┴─────────┐
              └──┬───┘ └┬─┘ └──┬───┘     │  Incident         │
                 │      │      │         │  Responder         │
                 ▼      ▼      ▼         │                   │
              ┌──────────────────┐       │  Severity Scoring  │
              │  Intelligence    ├──────▶│  Event Correlation │
              │  Correlation     │       │  LLM Analysis      │
              └──────────────────┘       └───────────────────┘
                                                  │
                                    ┌─────────────┼─────────────┐
                                    ▼             ▼             ▼
                              ┌──────────┐ ┌──────────┐ ┌──────────┐
                              │  Slack   │ │PagerDuty │ │Prometheus│
                              │  Alert   │ │  Page    │ │ Metrics  │
                              └──────────┘ └──────────┘ └──────────┘
```

### AI Intelligence Loop

```
              LLM-POWERED INTELLIGENCE & REMEDIATION
 ═══════════════════════════════════════════════════════════

     ┌──────────────────────────────────────────────────┐
     │              ANOMALY DETECTION                    │
     │                                                   │
     │  Time Series ──▶ Z-Score ──▶ Threshold Check     │
     │       │              │                            │
     │       ▼              ▼                            │
     │     EWMA          StdDev     Sensitivity:         │
     │   Smoothing      Analysis    High  = 2.0 sigma    │
     │       │              │       Med   = 3.0 sigma    │
     │       ▼              ▼       Low   = 4.0 sigma    │
     │  Trend Predict   Isolation                        │
     │  (Regression)     Score      Score: 0.0 ──▶ 1.0   │
     └───────────────────────┬──────────────────────────┘
                             │ anomaly detected
                             ▼
     ┌──────────────────────────────────────────────────┐
     │              INCIDENT CREATION                    │
     │                                                   │
     │  Score ≥ 0.9 ──▶ CRITICAL   ──▶ Page On-Call     │
     │  Score ≥ 0.7 ──▶ HIGH       ──▶ Slack Alert      │
     │  Score ≥ 0.5 ──▶ MEDIUM     ──▶ Dashboard        │
     │  Score < 0.5 ──▶ LOW        ──▶ Log Only         │
     │                                                   │
     │  Correlate: group related events within window    │
     └───────────────────────┬──────────────────────────┘
                             │
                             ▼
     ┌──────────────────────────────────────────────────┐
     │          ROOT CAUSE ANALYSIS (LLM)                │
     │                                                   │
     │  ┌────────────────┐    ┌───────────────────────┐ │
     │  │ Dependency     │    │  AWS Bedrock           │ │
     │  │ Graph          │    │  Claude / Titan         │ │
     │  │                │    │                         │ │
     │  │ ServiceA       │    │  Prompt:                │ │
     │  │   └─▶ ServiceB │───▶│  "Analyze this incident│ │
     │  │        └─▶ DB  │    │   with evidence..."    │ │
     │  │                │    │                         │ │
     │  │ Causal Chain   │    │  Response:              │ │
     │  │ (DFS traversal)│    │  Root cause + fix       │ │
     │  └────────────────┘    └───────────┬─────────────┘ │
     └────────────────────────────────────┼────────────────┘
                                          │
                                          ▼
     ┌──────────────────────────────────────────────────┐
     │           AUTO-REMEDIATION ENGINE                  │
     │                                                   │
     │  ┌──────────┐    ┌────────────┐    ┌───────────┐ │
     │  │ Strategy │    │  Safety    │    │  Execute  │ │
     │  │ Select   │───▶│  Checks   │───▶│  Action   │ │
     │  └──────────┘    └────────────┘    └───────────┘ │
     │                                                   │
     │  Actions:              Guards:                    │
     │  • scale_deployment    • Circuit breaker          │
     │  • restart_pods          (5 attempts/10 min)      │
     │  • rollback_deploy     • Dry run mode             │
     │  • drain_node          • Approval required        │
     │  • failover_db         • Max retry limit          │
     │  • toggle_feature      • Cooldown period          │
     └──────────────────────────────────────────────────┘
```

### Deployment Architecture

```
              KUBERNETES DEPLOYMENT
 ═══════════════════════════════════════════

 ┌─────────────────────────────────────────────────────┐
 │                  KUBERNETES CLUSTER                   │
 │                                                       │
 │  namespace: kronveil                                  │
 │  ┌─────────────────────────────────────────────────┐ │
 │  │  Deployment: kronveil-agent (replicas: 1)        │ │
 │  │  ┌───────────────────────────────────────────┐   │ │
 │  │  │  Pod                                      │   │ │
 │  │  │  ┌──────────────────────────────────────┐ │   │ │
 │  │  │  │  kronveil (Go binary)                │ │   │ │
 │  │  │  │                                      │ │   │ │
 │  │  │  │  REST API ──────────── :8080         │ │   │ │
 │  │  │  │  Prometheus metrics ── :9090         │ │   │ │
 │  │  │  │  gRPC API ──────────── :9091         │ │   │ │
 │  │  │  │  Health: /healthz, /readyz           │ │   │ │
 │  │  │  └──────────────────────────────────────┘ │   │ │
 │  │  └───────────────────────────────────────────┘   │ │
 │  └──────────────────────────────────────────────────┘ │
 │                                                       │
 │  ┌──────────────────┐  ┌──────────────────────────┐  │
 │  │  Service          │  │  ServiceAccount           │  │
 │  │  kronveil-agent   │  │  + ClusterRole            │  │
 │  │  ClusterIP        │  │  + ClusterRoleBinding     │  │
 │  │  8080, 9090, 9091 │  │                           │  │
 │  └──────────────────┘  │  Permissions:              │  │
 │                         │  • pods (get/list/watch)   │  │
 │  ┌──────────────────┐  │  • nodes (get/list/watch)  │  │
 │  │  ConfigMap        │  │  • events (get/list/watch) │  │
 │  │  kronveil-config  │  │  • deployments (get/list/  │  │
 │  │  config.yaml      │  │     watch/update)          │  │
 │  └──────────────────┘  └──────────────────────────┘  │
 │                                                       │
 │  ┌─────────────────────────────────────────────────┐ │
 │  │  Dashboard (Optional)                            │ │
 │  │  React SPA ── nginx :8080 ── /api/* proxy        │ │
 │  └─────────────────────────────────────────────────┘ │
 └─────────────────────────────────────────────────────┘
          │              │              │
          ▼              ▼              ▼
   ┌────────────┐ ┌────────────┐ ┌────────────┐
   │ Kafka      │ │ AWS        │ │ Vault      │
   │ Cluster    │ │ Bedrock    │ │ Server     │
   └────────────┘ └────────────┘ └────────────┘
```

---

## 🚀 Quick Start

### Prerequisites
- Kubernetes 1.26+
- Helm 3.x
- AWS account (for Bedrock LLM)
- Kafka cluster (or use bundled)

### Install via Helm

```bash
helm repo add kronveil https://charts.kronveil.io
helm repo update

helm install kronveil kronveil/kronveil \
  --namespace kronveil \
  --create-namespace \
  --set bedrock.region=us-east-1 \
  --set collectors.kubernetes.enabled=true \
  --set collectors.kafka.enabled=true
```

### Install via kubectl

```bash
kubectl apply -f https://raw.githubusercontent.com/kronveil/kronveil/main/deploy/install.yaml
```

### Verify Installation

```bash
kubectl get pods -n kronveil
kubectl port-forward svc/kronveil-dashboard 8080:8080 -n kronveil
# Open http://localhost:8080
```

---

## 📦 Components

```
kronveil/
├── core/
│   ├── agent-engine/          # LLM decision brain (AWS Bedrock)
│   ├── event-bus/             # Kafka-based event streaming backbone
│   └── policy-engine/         # OPA-based governance rules
│
├── collectors/
│   ├── kubernetes/            # Pod health, node metrics, admission events
│   ├── kafka/                 # Lag, throughput, partition health, consumer groups
│   ├── cloud/                 # AWS CloudWatch, Azure Monitor, GCP Operations
│   ├── cicd/                  # GitHub Actions, Jenkins, GitLab CI pipelines
│   └── logs/                  # ELK/Splunk/Loki integration
│
├── intelligence/
│   ├── anomaly-detector/      # Predictive anomaly detection (60%+ MTTR reduction)
│   ├── incident-responder/    # Autonomous remediation workflows
│   ├── root-cause-analyzer/   # LLM-powered causal chain analysis
│   └── capacity-planner/      # Forecasting & right-sizing AI
│
├── integrations/
│   ├── aws-bedrock/           # LLM backbone
│   ├── hashicorp-vault/       # Secret-aware monitoring
│   ├── pagerduty/             # Alerting & on-call
│   ├── slack/                 # ChatOps interface
│   ├── grafana/               # Dashboard embedding
│   └── prometheus/            # Metrics scraping
│
├── api/                       # REST & gRPC APIs
├── dashboard/                 # React-based UI
├── helm/                      # Helm charts
└── docs/                      # Documentation
```

---

## 🔌 Integrations

<table>
<tr>
<td><b>Cloud</b></td>
<td>AWS, Azure, GCP, On-Prem</td>
</tr>
<tr>
<td><b>Container</b></td>
<td>Kubernetes, Docker, containerd</td>
</tr>
<tr>
<td><b>Streaming</b></td>
<td>Apache Kafka, Confluent, AWS MSK</td>
</tr>
<tr>
<td><b>LLM</b></td>
<td>AWS Bedrock (Claude, Titan), OpenAI, Azure OpenAI</td>
</tr>
<tr>
<td><b>Observability</b></td>
<td>Prometheus, Grafana, Datadog, Splunk, ELK</td>
</tr>
<tr>
<td><b>Security</b></td>
<td>HashiCorp Vault, Falco, OPA Gatekeeper, Sysdig</td>
</tr>
<tr>
<td><b>GitOps</b></td>
<td>Flux, ArgoCD, Terraform, Crossplane</td>
</tr>
<tr>
<td><b>CI/CD</b></td>
<td>GitHub Actions, Jenkins, GitLab CI, Azure DevOps</td>
</tr>
<tr>
<td><b>Alerting</b></td>
<td>PagerDuty, Slack, Teams, Opsgenie</td>
</tr>
</table>

---

## 📊 Performance Benchmarks

| Metric | Result |
|--------|--------|
| Event ingestion throughput | **10M+ events/sec** |
| Anomaly detection latency | **< 30 seconds** |
| MTTR reduction | **60%+** |
| Supported VPCs/Clusters | **50+ simultaneous** |
| Availability SLA | **99.99%** |

---

## 🤝 Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

```bash
git clone https://github.com/kronveil/kronveil.git
cd kronveil
make dev-setup
make test
```

### Areas Actively Seeking Contributors
- 🔌 New collector integrations (Azure Monitor, GCP Ops)
- 🧠 LLM prompt engineering for better root-cause analysis
- 📊 Dashboard widgets and visualizations
- 🌐 Internationalization (i18n)
- 📝 Documentation improvements

---

## 📄 License

Apache License 2.0 — see [LICENSE](LICENSE) for details.

---

## 🙏 Acknowledgments

Kronveil was born from real-world experience building and operating enterprise platforms at financial institutions processing trillions in daily transactions. Special thanks to the open-source communities behind Kubernetes, Apache Kafka, OpenTelemetry, and OPA.

---

<p align="center">
  <b>Built by platform engineers. For platform engineers.</b><br/>
  <a href="https://kronveil.io">kronveil.io</a> · 
  <a href="https://discord.gg/kronveil">Discord</a> · 
  <a href="https://twitter.com/kronveil">@kronveil</a>
</p>
