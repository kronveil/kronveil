package policy

// DefaultPolicies contains built-in OPA Rego policies.
var DefaultPolicies = map[string]PolicyDefinition{
	"require-resource-limits": {
		ID:          "require-resource-limits",
		Name:        "Require Resource Limits",
		Description: "All pods must define CPU and memory resource limits",
		Severity:    "high",
		Category:    "kubernetes",
		Rego: `package kronveil.kubernetes.resource_limits

violation[msg] {
    input.kind == "Pod"
    container := input.spec.containers[_]
    not container.resources.limits.cpu
    msg := sprintf("Container '%s' in pod '%s' missing CPU limit", [container.name, input.metadata.name])
}

violation[msg] {
    input.kind == "Pod"
    container := input.spec.containers[_]
    not container.resources.limits.memory
    msg := sprintf("Container '%s' in pod '%s' missing memory limit", [container.name, input.metadata.name])
}
`,
	},
	"enforce-network-policy": {
		ID:          "enforce-network-policy",
		Name:        "Enforce Network Policy",
		Description: "Every namespace must have a default deny NetworkPolicy",
		Severity:    "critical",
		Category:    "security",
		Rego: `package kronveil.security.network_policy

violation[msg] {
    input.kind == "Namespace"
    namespace := input.metadata.name
    not has_default_deny(namespace)
    msg := sprintf("Namespace '%s' lacks a default deny NetworkPolicy", [namespace])
}

has_default_deny(ns) {
    some policy
    data.network_policies[policy].metadata.namespace == ns
    policy.spec.podSelector == {}
    count(policy.spec.ingress) == 0
}
`,
	},
	"no-latest-tag": {
		ID:          "no-latest-tag",
		Name:        "No Latest Image Tag",
		Description: "Container images must not use :latest tag",
		Severity:    "medium",
		Category:    "kubernetes",
		Rego: `package kronveil.kubernetes.no_latest

violation[msg] {
    input.kind == "Pod"
    container := input.spec.containers[_]
    endswith(container.image, ":latest")
    msg := sprintf("Container '%s' uses :latest tag for image '%s'", [container.name, container.image])
}

violation[msg] {
    input.kind == "Pod"
    container := input.spec.containers[_]
    not contains(container.image, ":")
    msg := sprintf("Container '%s' has no tag specified for image '%s' (implies :latest)", [container.name, container.image])
}
`,
	},
	"kafka-min-isr": {
		ID:          "kafka-min-isr",
		Name:        "Kafka Minimum ISR",
		Description: "Kafka topics must maintain min.insync.replicas >= 2",
		Severity:    "high",
		Category:    "kafka",
		Rego: `package kronveil.kafka.min_isr

violation[msg] {
    input.kind == "KafkaTopic"
    min_isr := to_number(input.config["min.insync.replicas"])
    min_isr < 2
    msg := sprintf("Topic '%s' has min.insync.replicas=%d (must be >= 2)", [input.metadata.name, min_isr])
}
`,
	},
	"secret-rotation": {
		ID:          "secret-rotation",
		Name:        "Secret Rotation Policy",
		Description: "All secrets must be rotated within 30-day window",
		Severity:    "medium",
		Category:    "compliance",
		Rego: `package kronveil.compliance.secret_rotation

import future.keywords.in

max_age_days := 30

violation[msg] {
    input.kind == "Secret"
    last_rotated := input.metadata.annotations["kronveil.io/last-rotated"]
    age_days := (time.now_ns() - time.parse_rfc3339_ns(last_rotated)) / (24 * 60 * 60 * 1000000000)
    age_days > max_age_days
    msg := sprintf("Secret '%s/%s' not rotated in %d days (max: %d)", [input.metadata.namespace, input.metadata.name, age_days, max_age_days])
}
`,
	},
}

// PolicyDefinition describes a governance policy.
type PolicyDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Rego        string `json:"rego"`
	Enabled     bool   `json:"enabled"`
}
