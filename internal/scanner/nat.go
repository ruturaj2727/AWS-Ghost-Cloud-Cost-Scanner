package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/cost"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// NATScanner scans for idle NAT Gateways
type NATScanner struct {
	client *aws.Client
	calc   *cost.Calculator
}

// NewNATScanner creates a new NAT Gateway scanner
func NewNATScanner(client *aws.Client) *NATScanner {
	return &NATScanner{
		client: client,
		calc:   cost.NewCalculator(),
	}
}

// Scan returns idle NAT Gateways
func (s *NATScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	var resources []types.Resource

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resp, err := s.client.EC2.DescribeNatGateways(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to describe NAT gateways: %w\nTip: Ensure you have ec2:DescribeNatGateways permission", err)
	}

	for _, nat := range resp.NatGateways {
		if nat.State == "deleted" || nat.State == "deleting" || nat.NatGatewayId == nil {
			continue
		}

		if s.isIdleNATGateway(*nat.NatGatewayId, config.IdleDays) {
			idleDays := s.calculateIdleDays(nat.CreateTime)

			vpcID := ""
			if nat.VpcId != nil {
				vpcID = *nat.VpcId
			}
			subnetID := ""
			if nat.SubnetId != nil {
				subnetID = *nat.SubnetId
			}

			resource := types.Resource{
				ID:          *nat.NatGatewayId,
				Type:        "nat",
				Region:      s.client.Config.Region,
				Name:        getNATName(nat.Tags),
				State:       string(nat.State),
				IdleDays:    idleDays,
				MonthlyCost: s.calc.NATGatewayCost(),
				Metadata: map[string]string{
					"vpc_id":    vpcID,
					"subnet_id": subnetID,
				},
				LastActive: time.Now().AddDate(0, 0, -idleDays),
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func (s *NATScanner) ResourceType() string {
	return "nat"
}

func (s *NATScanner) Description() string {
	return "Idle NAT Gateways"
}

func (s *NATScanner) isIdleNATGateway(natID string, idleDays int) bool {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -idleDays)
	period := int32(86400) // 1 day

	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  strPtr("AWS/NATGateway"),
		MetricName: strPtr("BytesOutToDestination"),
		Dimensions: []cwtypes.Dimension{
			{Name: strPtr("NatGatewayId"), Value: &natID},
		},
		StartTime:  &startTime,
		EndTime:    &endTime,
		Period:     &period,
		Statistics: []cwtypes.Statistic{cwtypes.StatisticSum},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := s.client.CloudWatch.GetMetricStatistics(ctx, input)
	if err != nil {
		// If we can't get metrics, don't flag it as idle (conservative approach)
		return false
	}

	// If no datapoints or all zeros, it's idle
	for _, dp := range resp.Datapoints {
		if dp.Sum != nil && *dp.Sum > 0 {
			return false
		}
	}

	return true
}

func (s *NATScanner) calculateIdleDays(createTime *time.Time) int {
	if createTime == nil {
		return 999
	}
	return int(time.Since(*createTime).Hours() / 24)
}

func getNATName(tags []ec2types.Tag) string {
	for _, tag := range tags {
		if tag.Key != nil && *tag.Key == "Name" && tag.Value != nil {
			return *tag.Value
		}
	}
	return ""
}

func strPtr(s string) *string {
	return &s
}
