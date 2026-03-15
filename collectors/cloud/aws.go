package cloud

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
)

// AWSProvider collects metrics from AWS CloudWatch.
type AWSProvider struct {
	regions       []string
	cwClients     map[string]*cloudwatch.Client
	tagClients    map[string]*resourcegroupstaggingapi.Client
}

// NewAWSProvider creates a new AWS cloud provider.
func NewAWSProvider(regions []string) *AWSProvider {
	p := &AWSProvider{
		regions:    regions,
		cwClients:  make(map[string]*cloudwatch.Client),
		tagClients: make(map[string]*resourcegroupstaggingapi.Client),
	}

	for _, region := range regions {
		cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(region),
		)
		if err != nil {
			log.Printf("[cloud-aws] WARNING: failed to load AWS config for %s: %v", region, err)
			continue
		}
		p.cwClients[region] = cloudwatch.NewFromConfig(cfg)
		p.tagClients[region] = resourcegroupstaggingapi.NewFromConfig(cfg)
	}

	return p
}

// Name returns the provider name.
func (a *AWSProvider) Name() string { return "aws" }

// metricQuery defines a CloudWatch metric to collect.
type metricQuery struct {
	service   string
	metric    string
	namespace string
	unit      string
	stat      string
}

var defaultMetricQueries = []metricQuery{
	{"EC2", "CPUUtilization", "AWS/EC2", "Percent", "Average"},
	{"EC2", "NetworkIn", "AWS/EC2", "Bytes", "Sum"},
	{"EC2", "NetworkOut", "AWS/EC2", "Bytes", "Sum"},
	{"RDS", "DatabaseConnections", "AWS/RDS", "Count", "Average"},
	{"RDS", "FreeStorageSpace", "AWS/RDS", "Bytes", "Average"},
	{"RDS", "CPUUtilization", "AWS/RDS", "Percent", "Average"},
	{"Lambda", "Invocations", "AWS/Lambda", "Count", "Sum"},
	{"Lambda", "Duration", "AWS/Lambda", "Milliseconds", "Average"},
	{"Lambda", "Errors", "AWS/Lambda", "Count", "Sum"},
	{"ELB", "RequestCount", "AWS/ApplicationELB", "Count", "Sum"},
	{"ELB", "TargetResponseTime", "AWS/ApplicationELB", "Seconds", "Average"},
	{"ELB", "HTTPCode_Target_5XX_Count", "AWS/ApplicationELB", "Count", "Sum"},
}

// CollectMetrics gathers CloudWatch metrics across configured regions.
func (a *AWSProvider) CollectMetrics(ctx context.Context) ([]CloudMetric, error) {
	var metrics []CloudMetric

	for _, region := range a.regions {
		cwClient, ok := a.cwClients[region]
		if !ok {
			// Fallback: emit stub metrics if no client for this region.
			for _, mq := range defaultMetricQueries {
				metrics = append(metrics, CloudMetric{
					Provider:   "aws",
					Region:     region,
					Service:    mq.service,
					Metric:     mq.metric,
					Unit:       mq.unit,
					Timestamp:  time.Now(),
					Dimensions: map[string]string{"region": region},
				})
			}
			continue
		}

		// Build GetMetricData queries.
		now := time.Now()
		startTime := now.Add(-5 * time.Minute)
		var queries []cwtypes.MetricDataQuery
		for i, mq := range defaultMetricQueries {
			queries = append(queries, cwtypes.MetricDataQuery{
				Id: aws.String(fmt.Sprintf("m%d", i)),
				MetricStat: &cwtypes.MetricStat{
					Metric: &cwtypes.Metric{
						MetricName: aws.String(mq.metric),
						Namespace:  aws.String(mq.namespace),
					},
					Period: aws.Int32(300),
					Stat:   aws.String(mq.stat),
				},
			})
		}

		output, err := cwClient.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
			StartTime:         &startTime,
			EndTime:           &now,
			MetricDataQueries: queries,
		})
		if err != nil {
			log.Printf("[cloud-aws] Failed to get CloudWatch metrics for %s: %v", region, err)
			// Fallback to stub metrics on error.
			for _, mq := range defaultMetricQueries {
				metrics = append(metrics, CloudMetric{
					Provider:   "aws",
					Region:     region,
					Service:    mq.service,
					Metric:     mq.metric,
					Unit:       mq.unit,
					Timestamp:  time.Now(),
					Dimensions: map[string]string{"region": region},
				})
			}
			continue
		}

		for _, result := range output.MetricDataResults {
			// Match back to our query definitions.
			for i, mq := range defaultMetricQueries {
				if aws.ToString(result.Id) == fmt.Sprintf("m%d", i) {
					var value float64
					if len(result.Values) > 0 {
						value = result.Values[0]
					}
					metrics = append(metrics, CloudMetric{
						Provider:   "aws",
						Region:     region,
						Service:    mq.service,
						Metric:     mq.metric,
						Value:      value,
						Unit:       mq.unit,
						Timestamp:  time.Now(),
						Dimensions: map[string]string{"region": region},
					})
					break
				}
			}
		}
	}

	return metrics, nil
}

// ListResources lists AWS resources across configured regions.
func (a *AWSProvider) ListResources(ctx context.Context) ([]CloudResource, error) {
	var resources []CloudResource

	for _, region := range a.regions {
		tagClient, ok := a.tagClients[region]
		if !ok {
			continue
		}

		paginator := resourcegroupstaggingapi.NewGetResourcesPaginator(tagClient,
			&resourcegroupstaggingapi.GetResourcesInput{})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				log.Printf("[cloud-aws] Failed to list resources in %s: %v", region, err)
				break
			}
			for _, mapping := range page.ResourceTagMappingList {
				tags := make(map[string]string)
				for _, t := range mapping.Tags {
					tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
				}
				resources = append(resources, CloudResource{
					Provider:   "aws",
					Region:     region,
					ResourceID: aws.ToString(mapping.ResourceARN),
					Tags:       tags,
				})
			}
		}
	}

	return resources, nil
}
