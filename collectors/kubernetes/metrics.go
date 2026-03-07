package kubernetes

import (
	"time"
)

// NodeMetrics represents resource metrics for a Kubernetes node.
type NodeMetrics struct {
	Name            string    `json:"name"`
	CPUUsage        float64   `json:"cpu_usage_cores"`
	CPUCapacity     float64   `json:"cpu_capacity_cores"`
	CPUPercent      float64   `json:"cpu_percent"`
	MemoryUsage     int64     `json:"memory_usage_bytes"`
	MemoryCapacity  int64     `json:"memory_capacity_bytes"`
	MemoryPercent   float64   `json:"memory_percent"`
	DiskUsage       int64     `json:"disk_usage_bytes"`
	DiskCapacity    int64     `json:"disk_capacity_bytes"`
	DiskPercent     float64   `json:"disk_percent"`
	PodCount        int       `json:"pod_count"`
	PodCapacity     int       `json:"pod_capacity"`
	Conditions      []string  `json:"conditions"`
	CollectedAt     time.Time `json:"collected_at"`
}

// PodMetrics represents resource usage for a Kubernetes pod.
type PodMetrics struct {
	Name            string             `json:"name"`
	Namespace       string             `json:"namespace"`
	Node            string             `json:"node"`
	Phase           string             `json:"phase"`
	Containers      []ContainerMetrics `json:"containers"`
	RestartCount    int32              `json:"restart_count"`
	CPURequest      float64            `json:"cpu_request_cores"`
	CPULimit        float64            `json:"cpu_limit_cores"`
	CPUUsage        float64            `json:"cpu_usage_cores"`
	MemoryRequest   int64              `json:"memory_request_bytes"`
	MemoryLimit     int64              `json:"memory_limit_bytes"`
	MemoryUsage     int64              `json:"memory_usage_bytes"`
	CollectedAt     time.Time          `json:"collected_at"`
}

// ContainerMetrics holds per-container metrics.
type ContainerMetrics struct {
	Name         string  `json:"name"`
	CPUUsage     float64 `json:"cpu_usage_cores"`
	MemoryUsage  int64   `json:"memory_usage_bytes"`
	RestartCount int32   `json:"restart_count"`
	Ready        bool    `json:"ready"`
	State        string  `json:"state"`
}

// HPAMetrics tracks HPA scaling events.
type HPAMetrics struct {
	Name             string    `json:"name"`
	Namespace        string    `json:"namespace"`
	CurrentReplicas  int32     `json:"current_replicas"`
	DesiredReplicas  int32     `json:"desired_replicas"`
	MinReplicas      int32     `json:"min_replicas"`
	MaxReplicas      int32     `json:"max_replicas"`
	CurrentCPU       int32     `json:"current_cpu_percent"`
	TargetCPU        int32     `json:"target_cpu_percent"`
	ScalingActive    bool      `json:"scaling_active"`
	LastScaleTime    time.Time `json:"last_scale_time"`
	CollectedAt      time.Time `json:"collected_at"`
}

// ResourceUtilization computes the utilization ratio for a pod.
func (p *PodMetrics) ResourceUtilization() (cpuRatio, memRatio float64) {
	if p.CPULimit > 0 {
		cpuRatio = p.CPUUsage / p.CPULimit
	}
	if p.MemoryLimit > 0 {
		memRatio = float64(p.MemoryUsage) / float64(p.MemoryLimit)
	}
	return
}

// IsOverProvisioned returns true if the pod is using less than 20% of its limits.
func (p *PodMetrics) IsOverProvisioned() bool {
	cpuRatio, memRatio := p.ResourceUtilization()
	return cpuRatio < 0.2 && memRatio < 0.2 && p.CPULimit > 0
}

// IsUnderProvisioned returns true if the pod is using more than 80% of its limits.
func (p *PodMetrics) IsUnderProvisioned() bool {
	cpuRatio, memRatio := p.ResourceUtilization()
	return cpuRatio > 0.8 || memRatio > 0.8
}
