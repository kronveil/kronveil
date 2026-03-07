package cloud

import (
	"context"
	"time"
)

// AWSProvider collects metrics from AWS CloudWatch.
type AWSProvider struct {
	regions []string
}

// NewAWSProvider creates a new AWS cloud provider.
func NewAWSProvider(regions []string) *AWSProvider {
	return &AWSProvider{regions: regions}
}

// Name returns the provider name.
func (a *AWSProvider) Name() string { return "aws" }

// CollectMetrics gathers CloudWatch metrics across configured regions.
func (a *AWSProvider) CollectMetrics(ctx context.Context) ([]CloudMetric, error) {
	var metrics []CloudMetric

	for _, region := range a.regions {
		// In production: uses AWS SDK v2 CloudWatch.GetMetricData for batched metric retrieval.
		// Collects EC2, RDS, Lambda, ECS, ELB, and S3 metrics.
		regionMetrics := []struct {
			service string
			metric  string
			unit    string
		}{
			{"EC2", "CPUUtilization", "Percent"},
			{"EC2", "NetworkIn", "Bytes"},
			{"EC2", "NetworkOut", "Bytes"},
			{"RDS", "DatabaseConnections", "Count"},
			{"RDS", "FreeStorageSpace", "Bytes"},
			{"RDS", "CPUUtilization", "Percent"},
			{"Lambda", "Invocations", "Count"},
			{"Lambda", "Duration", "Milliseconds"},
			{"Lambda", "Errors", "Count"},
			{"ELB", "RequestCount", "Count"},
			{"ELB", "TargetResponseTime", "Seconds"},
			{"ELB", "HTTPCode_Target_5XX_Count", "Count"},
		}

		for _, m := range regionMetrics {
			metrics = append(metrics, CloudMetric{
				Provider:   "aws",
				Region:     region,
				Service:    m.service,
				Metric:     m.metric,
				Unit:       m.unit,
				Timestamp:  time.Now(),
				Dimensions: map[string]string{"region": region},
			})
		}
	}

	return metrics, nil
}

// ListResources lists AWS resources across configured regions.
func (a *AWSProvider) ListResources(ctx context.Context) ([]CloudResource, error) {
	var resources []CloudResource

	for _, region := range a.regions {
		// In production: uses AWS Resource Groups Tagging API to list resources.
		_ = region
	}

	return resources, nil
}
