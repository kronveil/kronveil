package cloud

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"

	asset "cloud.google.com/go/asset/apiv1"
	"cloud.google.com/go/asset/apiv1/assetpb"

	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GCPProvider collects metrics from Google Cloud Monitoring.
type GCPProvider struct {
	regions       []string
	projectID     string
	metricClient  *monitoring.MetricClient
	assetClient   *asset.Client
}

// NewGCPProvider creates a new GCP cloud provider.
func NewGCPProvider(regions []string) *GCPProvider {
	projectID := os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	if projectID == "" {
		log.Printf("[cloud-gcp] WARNING: neither GCP_PROJECT_ID nor GOOGLE_CLOUD_PROJECT is set; GCP provider will return stub data")
	}

	p := &GCPProvider{
		regions:   regions,
		projectID: projectID,
	}

	ctx := context.Background()

	// Initialise Cloud Monitoring metric client (uses ADC automatically).
	mc, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		log.Printf("[cloud-gcp] WARNING: failed to create Monitoring MetricClient: %v (will use stub data)", err)
	} else {
		p.metricClient = mc
	}

	// Initialise Cloud Asset Inventory client (uses ADC automatically).
	ac, err := asset.NewClient(ctx)
	if err != nil {
		log.Printf("[cloud-gcp] WARNING: failed to create Asset client: %v (will use stub data)", err)
	} else {
		p.assetClient = ac
	}

	log.Printf("[cloud-gcp] GCP provider initialized (regions: %v, project: %s)", regions, projectID)
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

// stubMetrics returns metric definitions without real values (fallback).
func (g *GCPProvider) stubMetrics() []CloudMetric {
	var metrics []CloudMetric
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
	return metrics
}

// CollectMetrics gathers GCP Cloud Monitoring metrics across configured regions.
func (g *GCPProvider) CollectMetrics(ctx context.Context) ([]CloudMetric, error) {
	// Graceful fallback: if the monitoring client failed to initialize, return stub data.
	if g.metricClient == nil || g.projectID == "" {
		return g.stubMetrics(), nil
	}

	var metrics []CloudMetric

	now := time.Now()
	startTime := now.Add(-5 * time.Minute)

	for _, md := range defaultGCPMetrics {
		filter := fmt.Sprintf(`metric.type = "%s"`, md.metric)

		req := &monitoringpb.ListTimeSeriesRequest{
			Name:   fmt.Sprintf("projects/%s", g.projectID),
			Filter: filter,
			Interval: &monitoringpb.TimeInterval{
				StartTime: timestamppb.New(startTime),
				EndTime:   timestamppb.New(now),
			},
			View: monitoringpb.ListTimeSeriesRequest_FULL,
		}

		it := g.metricClient.ListTimeSeries(ctx, req)
		gotData := false
		for {
			ts, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				log.Printf("[cloud-gcp] Failed to read time series for %s: %v", md.metric, err)
				break
			}
			gotData = true

			// Extract the latest data point value.
			var value float64
			if len(ts.Points) > 0 {
				value = extractTypedValue(ts.Points[0].Value)
			}

			// Determine region from the monitored resource labels.
			region := ""
			if ts.Resource != nil {
				if z, ok := ts.Resource.Labels["zone"]; ok {
					region = z
				} else if r, ok := ts.Resource.Labels["region"]; ok {
					region = r
				}
			}

			dims := map[string]string{"project": g.projectID}
			if region != "" {
				dims["region"] = region
			}
			if ts.Resource != nil {
				for k, v := range ts.Resource.Labels {
					dims[k] = v
				}
			}

			metrics = append(metrics, CloudMetric{
				Provider:   "gcp",
				Region:     region,
				Service:    md.service,
				Metric:     md.metric,
				Value:      value,
				Unit:       md.unit,
				Timestamp:  time.Now(),
				Dimensions: dims,
			})
		}

		// If no data points returned for this metric, emit a stub so the pipeline has structure.
		if !gotData {
			for _, region := range g.regions {
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
	}

	return metrics, nil
}

// extractTypedValue extracts a float64 from a Cloud Monitoring TypedValue.
func extractTypedValue(tv *monitoringpb.TypedValue) float64 {
	if tv == nil {
		return 0
	}
	switch v := tv.Value.(type) {
	case *monitoringpb.TypedValue_DoubleValue:
		return v.DoubleValue
	case *monitoringpb.TypedValue_Int64Value:
		return float64(v.Int64Value)
	case *monitoringpb.TypedValue_BoolValue:
		if v.BoolValue {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// ListResources lists GCP resources across configured regions using Cloud Asset Inventory.
func (g *GCPProvider) ListResources(ctx context.Context) ([]CloudResource, error) {
	// Graceful fallback: if the asset client failed to initialize, return stub data.
	if g.assetClient == nil || g.projectID == "" {
		return g.stubResources(), nil
	}

	var resources []CloudResource

	req := &assetpb.SearchAllResourcesRequest{
		Scope: fmt.Sprintf("projects/%s", g.projectID),
	}

	it := g.assetClient.SearchAllResources(ctx, req)
	for {
		result, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("[cloud-gcp] Failed to search resources: %v (returning stub data)", err)
			return g.stubResources(), nil
		}

		tags := make(map[string]string)
		for k, v := range result.Labels {
			tags[k] = v
		}
		tags["provider"] = "gcp"

		region := result.Location
		if region == "" {
			region = "global"
		}

		resources = append(resources, CloudResource{
			Provider:   "gcp",
			Region:     region,
			ResourceID: result.Name,
			Service:    result.DisplayName,
			Type:       result.AssetType,
			Tags:       tags,
		})
	}

	// If no resources found, return stubs so pipeline has structure.
	if len(resources) == 0 {
		return g.stubResources(), nil
	}

	return resources, nil
}

// stubResources returns placeholder resource entries (fallback).
func (g *GCPProvider) stubResources() []CloudResource {
	var resources []CloudResource
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
	return resources
}
