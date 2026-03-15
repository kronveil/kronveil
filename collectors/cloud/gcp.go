package cloud

import (
	"context"
	"fmt"
	"log"
	"time"
)

// GCPProvider collects metrics from Google Cloud Monitoring.
type GCPProvider struct {
	regions   []string
	projectID string
}

// NewGCPProvider creates a new GCP cloud provider.
func NewGCPProvider(regions []string) *GCPProvider {
	p := &GCPProvider{
		regions: regions,
	}

	log.Printf("[cloud-gcp] GCP provider initialized (regions: %v)", regions)
	return p
}

// Name returns the provider name.
func (g *GCPProvider) Name() string { return "gcp" }

// gcpMetricDef defines a Cloud Monitoring metric to collect.
type gcpMetricDef struct {
	service string
	metric  string
	unit    string
}

var defaultGCPMetrics = []gcpMetricDef{
	{"ComputeEngine", "compute.googleapis.com/instance/cpu/utilization", "Percent"},
	{"ComputeEngine", "compute.googleapis.com/instance/network/received_bytes_count", "Bytes"},
	{"ComputeEngine", "compute.googleapis.com/instance/network/sent_bytes_count", "Bytes"},
	{"ComputeEngine", "compute.googleapis.com/instance/disk/read_bytes_count", "Bytes"},
	{"CloudSQL", "cloudsql.googleapis.com/database/cpu/utilization", "Percent"},
	{"CloudSQL", "cloudsql.googleapis.com/database/disk/utilization", "Percent"},
	{"CloudSQL", "cloudsql.googleapis.com/database/network/connections", "Count"},
	{"CloudFunctions", "cloudfunctions.googleapis.com/function/execution_count", "Count"},
	{"CloudFunctions", "cloudfunctions.googleapis.com/function/execution_times", "Milliseconds"},
	{"CloudRun", "run.googleapis.com/request_count", "Count"},
	{"CloudRun", "run.googleapis.com/request_latencies", "Milliseconds"},
	{"GKE", "kubernetes.io/node/cpu/allocatable_utilization", "Percent"},
	{"GKE", "kubernetes.io/node/memory/allocatable_utilization", "Percent"},
	{"GKE", "kubernetes.io/pod/network/received_bytes_count", "Bytes"},
}

// CollectMetrics gathers GCP Cloud Monitoring metrics across configured regions.
func (g *GCPProvider) CollectMetrics(ctx context.Context) ([]CloudMetric, error) {
	var metrics []CloudMetric

	// TODO: Wire real Cloud Monitoring SDK when cloud.google.com/go/monitoring is added.
	// For now, emit metric definitions so the pipeline has structure.
	for _, region := range g.regions {
		for _, md := range defaultGCPMetrics {
			metrics = append(metrics, CloudMetric{
				Provider:   "gcp",
				Region:     region,
				Service:    md.service,
				Metric:     md.metric,
				Unit:       md.unit,
				Timestamp:  time.Now(),
				Dimensions: map[string]string{"region": region, "project": g.projectID},
			})
		}
	}

	return metrics, nil
}

// ListResources lists GCP resources across configured regions.
func (g *GCPProvider) ListResources(ctx context.Context) ([]CloudResource, error) {
	var resources []CloudResource

	// TODO: Wire real Cloud Asset Inventory SDK.
	for _, region := range g.regions {
		resources = append(resources, CloudResource{
			Provider:   "gcp",
			Region:     region,
			ResourceID: fmt.Sprintf("projects/%s", g.projectID),
			Service:    "project",
			Type:       "cloudresourcemanager.googleapis.com/Project",
			Tags:       map[string]string{"provider": "gcp"},
		})
	}

	return resources, nil
}
