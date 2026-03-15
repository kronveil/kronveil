package config

import (
	"fmt"
	"time"
)

// Config holds the complete Kronveil agent configuration.
type Config struct {
	Agent        AgentConfig        `yaml:"agent" json:"agent"`
	Kafka        KafkaConfig        `yaml:"kafka" json:"kafka"`
	Bedrock      BedrockConfig      `yaml:"bedrock" json:"bedrock"`
	Collectors   CollectorsConfig   `yaml:"collectors" json:"collectors"`
	Intelligence IntelligenceConfig `yaml:"intelligence" json:"intelligence"`
	Integrations IntegrationsConfig `yaml:"integrations" json:"integrations"`
	API          APIConfig          `yaml:"api" json:"api"`
}

type AgentConfig struct {
	Name       string `yaml:"name" json:"name"`
	ClusterID  string `yaml:"cluster_id" json:"cluster_id"`
	LogLevel   string `yaml:"log_level" json:"log_level"`
	LogFormat  string `yaml:"log_format" json:"log_format"`
}

type KafkaConfig struct {
	BootstrapServers string `yaml:"bootstrap_servers" json:"bootstrap_servers"`
	GroupID          string `yaml:"group_id" json:"group_id"`
	SecurityProtocol string `yaml:"security_protocol" json:"security_protocol"`
}

type BedrockConfig struct {
	Region      string  `yaml:"region" json:"region"`
	ModelID     string  `yaml:"model_id" json:"model_id"`
	MaxTokens   int     `yaml:"max_tokens" json:"max_tokens"`
	Temperature float64 `yaml:"temperature" json:"temperature"`
}

type CollectorsConfig struct {
	Kubernetes KubernetesCollectorConfig `yaml:"kubernetes" json:"kubernetes"`
	Kafka      KafkaCollectorConfig      `yaml:"kafka" json:"kafka"`
	Cloud      CloudCollectorConfig      `yaml:"cloud" json:"cloud"`
	CICD       CICDCollectorConfig       `yaml:"cicd" json:"cicd"`
	Logs       LogsCollectorConfig       `yaml:"logs" json:"logs"`
}

type KubernetesCollectorConfig struct {
	Enabled      bool          `yaml:"enabled" json:"enabled"`
	Kubeconfig   string        `yaml:"kubeconfig" json:"kubeconfig"`
	Namespaces   []string      `yaml:"namespaces" json:"namespaces"`
	PollInterval time.Duration `yaml:"poll_interval" json:"poll_interval"`
}

type KafkaCollectorConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	BootstrapServers string        `yaml:"bootstrap_servers" json:"bootstrap_servers"`
	MonitoredTopics  []string      `yaml:"monitored_topics" json:"monitored_topics"`
	ConsumerGroups   []string      `yaml:"consumer_groups" json:"consumer_groups"`
	LagThreshold     int64         `yaml:"lag_threshold" json:"lag_threshold"`
	PollInterval     time.Duration `yaml:"poll_interval" json:"poll_interval"`
}

type CloudCollectorConfig struct {
	Enabled  bool     `yaml:"enabled" json:"enabled"`
	Provider string   `yaml:"provider" json:"provider"`
	Regions  []string `yaml:"regions" json:"regions"`
}

type CICDCollectorConfig struct {
	Enabled     bool     `yaml:"enabled" json:"enabled"`
	WebhookPort int      `yaml:"webhook_port" json:"webhook_port"`
	RepoFilters []string `yaml:"repo_filters" json:"repo_filters"`
}

type LogsCollectorConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	ErrorPatterns []string `yaml:"error_patterns" json:"error_patterns"`
	ParseFormat   string   `yaml:"parse_format" json:"parse_format"`
}

type IntelligenceConfig struct {
	AnomalyDetection AnomalyDetectionConfig `yaml:"anomaly_detection" json:"anomaly_detection"`
	Remediation      RemediationConfig      `yaml:"remediation" json:"remediation"`
}

type AnomalyDetectionConfig struct {
	Sensitivity     string  `yaml:"sensitivity" json:"sensitivity"`
	WindowSize      int     `yaml:"window_size" json:"window_size"`
	ZScoreThreshold float64 `yaml:"zscore_threshold" json:"zscore_threshold"`
}

type RemediationConfig struct {
	AutoRemediate bool `yaml:"auto_remediate" json:"auto_remediate"`
	DryRun        bool `yaml:"dry_run" json:"dry_run"`
	MaxRetries    int  `yaml:"max_retries" json:"max_retries"`
}

type IntegrationsConfig struct {
	Vault         VaultIntegrationConfig         `yaml:"vault" json:"vault"`
	AWSSecrets    AWSSecretsIntegrationConfig    `yaml:"aws_secrets" json:"aws_secrets"`
	Slack         SlackIntegrationConfig         `yaml:"slack" json:"slack"`
	PagerDuty     PagerDutyIntegrationConfig     `yaml:"pagerduty" json:"pagerduty"`
	Prometheus    PrometheusIntegrationConfig    `yaml:"prometheus" json:"prometheus"`
	OpenTelemetry OTelIntegrationConfig          `yaml:"opentelemetry" json:"opentelemetry"`
}

type VaultIntegrationConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Address    string `yaml:"address" json:"address"`
	AuthMethod string `yaml:"auth_method" json:"auth_method"`
}

type AWSSecretsIntegrationConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	Region         string `yaml:"region" json:"region"`
	SecretPrefix   string `yaml:"secret_prefix" json:"secret_prefix"`
	RotationWindow string `yaml:"rotation_window" json:"rotation_window"`
	CacheEnabled   bool   `yaml:"cache_enabled" json:"cache_enabled"`
}

type SlackIntegrationConfig struct {
	Enabled        bool              `yaml:"enabled" json:"enabled"`
	BotToken       string            `yaml:"bot_token" json:"bot_token"`
	DefaultChannel string            `yaml:"default_channel" json:"default_channel"`
	Channels       map[string]string `yaml:"channels" json:"channels"`
}

type PagerDutyIntegrationConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	RoutingKey string `yaml:"routing_key" json:"routing_key"`
}

type PrometheusIntegrationConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	Port        int    `yaml:"port" json:"port"`
	MetricsPath string `yaml:"metrics_path" json:"metrics_path"`
}

type OTelIntegrationConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	Endpoint       string `yaml:"endpoint" json:"endpoint"`
	Insecure       bool   `yaml:"insecure" json:"insecure"`
	ExportInterval string `yaml:"export_interval" json:"export_interval"`
}

type APIConfig struct {
	RESTPort int    `yaml:"rest_port" json:"rest_port"`
	GRPCPort int    `yaml:"grpc_port" json:"grpc_port"`
	APIKey   string `yaml:"api_key" json:"api_key"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Agent: AgentConfig{
			Name:      "kronveil-agent",
			LogLevel:  "info",
			LogFormat: "json",
		},
		Kafka: KafkaConfig{
			BootstrapServers: "localhost:9092",
			GroupID:          "kronveil-agent",
		},
		Bedrock: BedrockConfig{
			Region:      "us-east-1",
			ModelID:     "anthropic.claude-3-sonnet-20240229-v1:0",
			MaxTokens:   2048,
			Temperature: 0.3,
		},
		Collectors: CollectorsConfig{
			Kubernetes: KubernetesCollectorConfig{
				Enabled:      true,
				PollInterval: 30 * time.Second,
			},
			Kafka: KafkaCollectorConfig{
				Enabled:      true,
				LagThreshold: 10000,
				PollInterval: 10 * time.Second,
			},
		},
		Intelligence: IntelligenceConfig{
			AnomalyDetection: AnomalyDetectionConfig{
				Sensitivity:     "medium",
				WindowSize:      300,
				ZScoreThreshold: 3.0,
			},
			Remediation: RemediationConfig{
				AutoRemediate: false,
				DryRun:        true,
				MaxRetries:    3,
			},
		},
		Integrations: IntegrationsConfig{
			Prometheus: PrometheusIntegrationConfig{
				Enabled:     true,
				Port:        9090,
				MetricsPath: "/metrics",
			},
			OpenTelemetry: OTelIntegrationConfig{
				Enabled:        false,
				Endpoint:       "localhost:4317",
				Insecure:       false,
				ExportInterval: "30s",
			},
		},
		API: APIConfig{
			RESTPort: 8080,
			GRPCPort: 9091,
		},
	}
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if c.Kafka.BootstrapServers == "" {
		return fmt.Errorf("kafka.bootstrap_servers is required")
	}
	if c.Bedrock.Region == "" {
		return fmt.Errorf("bedrock.region is required")
	}
	if c.API.RESTPort <= 0 || c.API.RESTPort > 65535 {
		return fmt.Errorf("api.rest_port must be between 1 and 65535")
	}
	return nil
}
