package eventbus

// Standard Kronveil Kafka topics.
const (
	TopicTelemetryK8s    = "kronveil.telemetry.kubernetes"
	TopicTelemetryKafka  = "kronveil.telemetry.kafka"
	TopicTelemetryCloud  = "kronveil.telemetry.cloud"
	TopicTelemetryCICD   = "kronveil.telemetry.cicd"
	TopicTelemetryLogs   = "kronveil.telemetry.logs"
	TopicAnomalies       = "kronveil.intelligence.anomalies"
	TopicIncidents       = "kronveil.intelligence.incidents"
	TopicRemediations    = "kronveil.intelligence.remediations"
	TopicPolicyViolations = "kronveil.policy.violations"
	TopicAgentHealth     = "kronveil.agent.health"
)

// AllTopics returns all standard Kronveil topics for initialization.
func AllTopics() []TopicConfig {
	return []TopicConfig{
		{Name: TopicTelemetryK8s, Partitions: 12, ReplicationFactor: 3, RetentionMs: 86400000},
		{Name: TopicTelemetryKafka, Partitions: 6, ReplicationFactor: 3, RetentionMs: 86400000},
		{Name: TopicTelemetryCloud, Partitions: 6, ReplicationFactor: 3, RetentionMs: 86400000},
		{Name: TopicTelemetryCICD, Partitions: 3, ReplicationFactor: 3, RetentionMs: 604800000},
		{Name: TopicTelemetryLogs, Partitions: 12, ReplicationFactor: 3, RetentionMs: 259200000},
		{Name: TopicAnomalies, Partitions: 6, ReplicationFactor: 3, RetentionMs: 604800000},
		{Name: TopicIncidents, Partitions: 3, ReplicationFactor: 3, RetentionMs: 2592000000},
		{Name: TopicRemediations, Partitions: 3, ReplicationFactor: 3, RetentionMs: 2592000000},
		{Name: TopicPolicyViolations, Partitions: 3, ReplicationFactor: 3, RetentionMs: 2592000000},
		{Name: TopicAgentHealth, Partitions: 1, ReplicationFactor: 3, RetentionMs: 86400000},
	}
}

// TopicConfig describes the desired configuration for a Kafka topic.
type TopicConfig struct {
	Name              string
	Partitions        int
	ReplicationFactor int
	RetentionMs       int64
}
