package cloud

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/azquery"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

// AzureProvider collects metrics from Azure Monitor.
type AzureProvider struct {
	regions        []string
	subscriptionID string
	metricsClient  *azquery.MetricsClient
	resourceClient *armresources.Client
}

// NewAzureProvider creates a new Azure cloud provider.
func NewAzureProvider(regions []string) *AzureProvider {
	p := &AzureProvider{
		regions:        regions,
		subscriptionID: os.Getenv("AZURE_SUBSCRIPTION_ID"),
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Printf("[cloud-azure] WARNING: failed to create Azure credential: %v (falling back to stub data)", err)
		return p
	}

	metricsClient, err := azquery.NewMetricsClient(cred, nil)
	if err != nil {
		log.Printf("[cloud-azure] WARNING: failed to create Azure metrics client: %v (falling back to stub data)", err)
		return p
	}
	p.metricsClient = metricsClient

	if p.subscriptionID != "" {
		resClient, err := armresources.NewClient(p.subscriptionID, cred, nil)
		if err != nil {
			log.Printf("[cloud-azure] WARNING: failed to create Azure resources client: %v (falling back to stub data)", err)
		} else {
			p.resourceClient = resClient
		}
	} else {
		log.Printf("[cloud-azure] WARNING: AZURE_SUBSCRIPTION_ID not set, resource listing will use stub data")
	}

	log.Printf("[cloud-azure] Azure provider initialized (regions: %v)", regions)
	return p
}

// Name returns the provider name.
func (a *AzureProvider) Name() string { return "azure" }

// azureMetricDef defines an Azure Monitor metric to collect.
type azureMetricDef struct {
	service   string
	metric    string
	namespace string
	unit      string
}

var defaultAzureMetrics = []azureMetricDef{
	{"VirtualMachines", "Percentage CPU", "Microsoft.Compute/virtualMachines", "Percent"},
	{"VirtualMachines", "Network In Total", "Microsoft.Compute/virtualMachines", "Bytes"},
	{"VirtualMachines", "Network Out Total", "Microsoft.Compute/virtualMachines", "Bytes"},
	{"VirtualMachines", "Disk Read Bytes", "Microsoft.Compute/virtualMachines", "Bytes"},
	{"VirtualMachines", "Disk Write Bytes", "Microsoft.Compute/virtualMachines", "Bytes"},
	{"SQLDatabase", "cpu_percent", "Microsoft.Sql/servers/databases", "Percent"},
	{"SQLDatabase", "dtu_consumption_percent", "Microsoft.Sql/servers/databases", "Percent"},
	{"SQLDatabase", "storage_percent", "Microsoft.Sql/servers/databases", "Percent"},
	{"AppService", "CpuPercentage", "Microsoft.Web/sites", "Percent"},
	{"AppService", "Http5xx", "Microsoft.Web/sites", "Count"},
	{"AppService", "HttpResponseTime", "Microsoft.Web/sites", "Seconds"},
	{"AKS", "node_cpu_usage_percentage", "Microsoft.ContainerService/managedClusters", "Percent"},
	{"AKS", "node_memory_rss_percentage", "Microsoft.ContainerService/managedClusters", "Percent"},
	{"Functions", "FunctionExecutionCount", "Microsoft.Web/sites", "Count"},
}

// CollectMetrics gathers Azure Monitor metrics across configured regions.
func (a *AzureProvider) CollectMetrics(ctx context.Context) ([]CloudMetric, error) {
	var metrics []CloudMetric

	if a.metricsClient == nil || a.subscriptionID == "" {
		// Fallback: emit stub metrics when SDK client is unavailable.
		for _, region := range a.regions {
			for _, md := range defaultAzureMetrics {
				metrics = append(metrics, CloudMetric{
					Provider:   "azure",
					Region:     region,
					Service:    md.service,
					Metric:     md.metric,
					Unit:       md.unit,
					Timestamp:  time.Now(),
					Dimensions: map[string]string{"region": region, "namespace": md.namespace},
				})
			}
		}
		return metrics, nil
	}

	// Group metric definitions by namespace so we can batch queries per resource type.
	nsByNamespace := make(map[string][]azureMetricDef)
	for _, md := range defaultAzureMetrics {
		nsByNamespace[md.namespace] = append(nsByNamespace[md.namespace], md)
	}

	// List resources to get concrete resource IDs for metric queries.
	resources, err := a.ListResources(ctx)
	if err != nil {
		log.Printf("[cloud-azure] WARNING: failed to list resources for metric collection: %v", err)
	}

	// Build a map of namespace -> []resourceID for targeted metric queries.
	resourcesByType := make(map[string][]string)
	for _, res := range resources {
		resourcesByType[res.Type] = append(resourcesByType[res.Type], res.ResourceID)
	}

	now := time.Now()
	startTime := now.Add(-5 * time.Minute)
	timespan := azquery.TimeInterval(fmt.Sprintf("%s/%s",
		startTime.UTC().Format(time.RFC3339),
		now.UTC().Format(time.RFC3339),
	))

	for namespace, defs := range nsByNamespace {
		resourceIDs, ok := resourcesByType[namespace]
		if !ok || len(resourceIDs) == 0 {
			// No resources of this type found; emit stubs for these metrics.
			for _, region := range a.regions {
				for _, md := range defs {
					metrics = append(metrics, CloudMetric{
						Provider:   "azure",
						Region:     region,
						Service:    md.service,
						Metric:     md.metric,
						Unit:       md.unit,
						Timestamp:  time.Now(),
						Dimensions: map[string]string{"region": region, "namespace": md.namespace},
					})
				}
			}
			continue
		}

		// Build comma-separated metric names for this namespace.
		metricNames := make([]string, len(defs))
		for i, d := range defs {
			metricNames[i] = d.metric
		}
		metricNamesStr := strings.Join(metricNames, ",")

		for _, resourceID := range resourceIDs {
			avgAgg := azquery.AggregationTypeAverage
			resp, err := a.metricsClient.QueryResource(ctx, resourceID, &azquery.MetricsClientQueryResourceOptions{
				Timespan:        &timespan,
				Interval:        to.Ptr("PT5M"),
				MetricNames:     &metricNamesStr,
				Aggregation:     []*azquery.AggregationType{&avgAgg},
				MetricNamespace: &namespace,
			})
			if err != nil {
				log.Printf("[cloud-azure] Failed to query metrics for %s: %v", resourceID, err)
				continue
			}

			// Extract region from resource ID (format: /subscriptions/.../resourceGroups/.../providers/...).
			region := extractRegionFromResourceID(resourceID)

			for _, metricValue := range resp.Value {
				if metricValue.Name == nil || metricValue.Name.Value == nil {
					continue
				}
				name := *metricValue.Name.Value

				// Find the matching metric definition.
				var matchedDef *azureMetricDef
				for i := range defs {
					if defs[i].metric == name {
						matchedDef = &defs[i]
						break
					}
				}
				if matchedDef == nil {
					continue
				}

				// Get the most recent data point.
				var value float64
				for _, ts := range metricValue.TimeSeries {
					for _, dp := range ts.Data {
						if dp.Average != nil {
							value = *dp.Average
						}
					}
				}

				metrics = append(metrics, CloudMetric{
					Provider:  "azure",
					Region:    region,
					Service:   matchedDef.service,
					Metric:    matchedDef.metric,
					Value:     value,
					Unit:      matchedDef.unit,
					Timestamp: time.Now(),
					Dimensions: map[string]string{
						"region":      region,
						"namespace":   namespace,
						"resource_id": resourceID,
					},
				})
			}
		}
	}

	return metrics, nil
}

// ListResources lists Azure resources across configured regions.
func (a *AzureProvider) ListResources(ctx context.Context) ([]CloudResource, error) {
	var resources []CloudResource

	if a.resourceClient == nil {
		// Fallback: emit stub resources when SDK client is unavailable.
		for _, region := range a.regions {
			resources = append(resources, CloudResource{
				Provider:   "azure",
				Region:     region,
				ResourceID: fmt.Sprintf("/subscriptions/%s/resourceGroups/default", a.subscriptionID),
				Service:    "resource-group",
				Type:       "Microsoft.Resources/resourceGroups",
				Tags:       map[string]string{"provider": "azure"},
			})
		}
		return resources, nil
	}

	for _, region := range a.regions {
		filter := fmt.Sprintf("location eq '%s'", region)
		pager := a.resourceClient.NewListPager(&armresources.ClientListOptions{
			Filter: &filter,
		})

		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				log.Printf("[cloud-azure] Failed to list resources in %s: %v", region, err)
				break
			}
			for _, res := range page.Value {
				tags := make(map[string]string)
				for k, v := range res.Tags {
					if v != nil {
						tags[k] = *v
					}
				}

				var resourceType, service string
				if res.Type != nil {
					resourceType = *res.Type
					// Derive a short service name from the resource type.
					parts := strings.Split(resourceType, "/")
					if len(parts) >= 2 {
						service = parts[len(parts)-1]
					} else {
						service = resourceType
					}
				}

				var resourceID string
				if res.ID != nil {
					resourceID = *res.ID
				}

				resources = append(resources, CloudResource{
					Provider:   "azure",
					Region:     region,
					ResourceID: resourceID,
					Service:    service,
					Type:       resourceType,
					Tags:       tags,
				})
			}
		}
	}

	return resources, nil
}

// extractRegionFromResourceID attempts to infer a region hint from the resource ID.
// Azure resource IDs don't embed region, so this returns "unknown" as a default.
// The actual region is known from the ListResources filter.
func extractRegionFromResourceID(_ string) string {
	return "unknown"
}
