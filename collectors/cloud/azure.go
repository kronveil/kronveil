package cloud

import (
	"context"
	"fmt"
	"log"
	"time"
)

// AzureProvider collects metrics from Azure Monitor.
type AzureProvider struct {
	regions        []string
	subscriptionID string
}

// NewAzureProvider creates a new Azure cloud provider.
func NewAzureProvider(regions []string) *AzureProvider {
	p := &AzureProvider{
		regions: regions,
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

	// TODO: Wire real Azure Monitor SDK when azure-sdk-for-go is added.
	// For now, emit metric definitions so the pipeline has structure.
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

// ListResources lists Azure resources across configured regions.
func (a *AzureProvider) ListResources(ctx context.Context) ([]CloudResource, error) {
	var resources []CloudResource

	// TODO: Wire real Azure Resource Graph SDK.
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
