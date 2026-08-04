package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// CloudFrontScanner scans for wasteful CloudFront distributions
type CloudFrontScanner struct {
	cfClient *cloudfront.Client
	cwClient *cloudwatch.Client
}

// NewCloudFrontScanner creates a new CloudFront scanner
func NewCloudFrontScanner(cfClient, cwClient interface{}) types.Scanner {
	return &CloudFrontScanner{
		cfClient: cfClient.(*cloudfront.Client),
		cwClient: cwClient.(*cloudwatch.Client),
	}
}

// ResourceType returns the type identifier for this scanner
func (c *CloudFrontScanner) ResourceType() string {
	return "CloudFront Distribution"
}

// Description returns a human-readable description
func (c *CloudFrontScanner) Description() string {
	return "Scans for wasteful CloudFront distributions including disabled and idle distributions"
}

// Scan returns wasteful CloudFront resources
func (c *CloudFrontScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	ctx := context.Background()
	var resources []types.Resource

	// CloudFront is global, but we'll include it in all region scans
	distributions, err := c.listDistributions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list CloudFront distributions: %w", err)
	}

	for _, dist := range distributions {
		// Check for disabled distributions
		if dist.Enabled == nil || !*dist.Enabled {
			cost := c.calculateDisabledDistributionCost(dist)
			resources = append(resources, types.Resource{
				ID:          *dist.Id,
				Type:        "CloudFront Distribution",
				Region:      "global",
				State:       "Disabled",
				IdleDays:    0,
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": "Distribution is disabled but still configured"},
				LastActive:  time.Time{},
			})
			continue
		}

		// Check for distributions with zero traffic
		hasTraffic, err := c.checkDistributionTraffic(ctx, *dist.Id, config.IdleDays)
		if err != nil {
			continue
		}

		if !hasTraffic {
			cost := c.calculateIdleDistributionCost(dist)
			resources = append(resources, types.Resource{
				ID:          *dist.Id,
				Type:        "CloudFront Distribution",
				Region:      "global",
				State:       "Idle",
				IdleDays:    config.IdleDays,
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": "No traffic for specified period"},
				LastActive:  time.Time{},
			})
		}

		// Check for distributions with old SSL certificates
		if err := c.checkOldSSLCertificates(ctx, dist); err == nil {
			cost := c.calculateIdleDistributionCost(dist) * 0.5 // Additional cost for security risk
			resources = append(resources, types.Resource{
				ID:          *dist.Id,
				Type:        "CloudFront Distribution",
				Region:      "global",
				State:       "Old SSL certificate",
				IdleDays:    0,
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": "Using SSL certificate that is nearing expiration"},
				LastActive:  time.Time{},
			})
		}
	}

	return resources, nil
}

// listDistributions lists all CloudFront distributions
func (c *CloudFrontScanner) listDistributions(ctx context.Context) ([]cloudfronttypes.DistributionSummary, error) {
	var distributions []cloudfronttypes.DistributionSummary

	result, err := c.cfClient.ListDistributions(ctx, &cloudfront.ListDistributionsInput{})
	if err != nil {
		return nil, err
	}

	if result.DistributionList != nil {
		distributions = append(distributions, result.DistributionList.Items...)
	}

	return distributions, nil
}

// checkDistributionTraffic checks if distribution has had traffic in the specified days
func (c *CloudFrontScanner) checkDistributionTraffic(ctx context.Context, distributionID string, idleDays int) (bool, error) {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -idleDays)

	// Check requests
	requestsInput := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/CloudFront"),
		MetricName: aws.String("Requests"),
		Dimensions: []cloudwatchtypes.Dimension{
			{
				Name:  aws.String("DistributionId"),
				Value: aws.String(distributionID),
			},
		},
		StartTime:  &startTime,
		EndTime:    &endTime,
		Period:     aws.Int32(86400), // 24 hours
		Statistics: []cloudwatchtypes.Statistic{cloudwatchtypes.StatisticSum},
	}

	requestsResult, err := c.cwClient.GetMetricStatistics(ctx, requestsInput)
	if err != nil {
		return false, err
	}

	// Check data transferred
	dataInput := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/CloudFront"),
		MetricName: aws.String("BytesDownloaded"),
		Dimensions: []cloudwatchtypes.Dimension{
			{
				Name:  aws.String("DistributionId"),
				Value: aws.String(distributionID),
			},
		},
		StartTime:  &startTime,
		EndTime:    &endTime,
		Period:     aws.Int32(86400), // 24 hours
		Statistics: []cloudwatchtypes.Statistic{cloudwatchtypes.StatisticSum},
	}

	dataResult, err := c.cwClient.GetMetricStatistics(ctx, dataInput)
	if err != nil {
		return false, err
	}

	// Check if there was any activity
	hasRequests := len(requestsResult.Datapoints) > 0
	hasData := len(dataResult.Datapoints) > 0

	return hasRequests || hasData, nil
}

// checkOldSSLCertificates checks for distributions using old SSL certificates
func (c *CloudFrontScanner) checkOldSSLCertificates(ctx context.Context, dist cloudfronttypes.DistributionSummary) error {
	// TODO: Implement SSL certificate check with correct AWS SDK v2 API
	// The ViewerCertificate field structure has changed in AWS SDK v2
	// For now, return error to skip this check
	return fmt.Errorf("SSL certificate check not yet implemented")
}

// calculateDisabledDistributionCost calculates cost for disabled distribution
func (c *CloudFrontScanner) calculateDisabledDistributionCost(dist cloudfronttypes.DistributionSummary) float64 {
	// Disabled distributions still incur small costs for configuration storage
	return 0.50 // $0.50 per month for disabled distribution
}

// calculateIdleDistributionCost calculates cost for idle distribution
func (c *CloudFrontScanner) calculateIdleDistributionCost(dist cloudfronttypes.DistributionSummary) float64 {
	// Base cost for CloudFront distribution
	baseCost := 0.50 // $0.50 per month for distribution

	// TODO: Add SSL certificate cost calculation when API is updated
	// The ViewerCertificate field structure has changed in AWS SDK v2

	// Add alternate domain name costs
	if len(dist.Aliases.Items) > 0 {
		baseCost += float64(len(dist.Aliases.Items)) * 0.10
	}

	return baseCost
}
