package scanner

import (
	"context"
	"strings"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/cost"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// LoadBalancerScanner scans for idle load balancers
type LoadBalancerScanner struct {
	client *aws.Client
	calc   *cost.Calculator
}

// NewLoadBalancerScanner creates a new load balancer scanner
func NewLoadBalancerScanner(client *aws.Client) *LoadBalancerScanner {
	return &LoadBalancerScanner{
		client: client,
		calc:   cost.NewCalculator(),
	}
}

// Scan returns idle load balancers (ALB/NLB/CLB)
func (s *LoadBalancerScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	var resources []types.Resource

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Scan ALBs and NLBs
	albResp, err := s.client.ELBv2.DescribeLoadBalancers(ctx, nil)
	if err == nil {
		for _, lb := range albResp.LoadBalancers {
			if lb.LoadBalancerArn == nil || lb.LoadBalancerName == nil {
				continue
			}
			if s.isIdleLoadBalancer(*lb.LoadBalancerArn, config.IdleDays) {
				idleDays := s.calculateIdleDays(lb.CreatedTime)

				dnsName := ""
				if lb.DNSName != nil {
					dnsName = *lb.DNSName
				}

				resource := types.Resource{
					ID:          *lb.LoadBalancerArn,
					Type:        "loadbalancer",
					Region:      s.client.Config.Region,
					Name:        *lb.LoadBalancerName,
					State:       string(lb.State.Code),
					IdleDays:    idleDays,
					MonthlyCost: s.calc.LoadBalancerCost(string(lb.Type)),
					Metadata: map[string]string{
						"type":     string(lb.Type),
						"scheme":   string(lb.Scheme),
						"dns_name": dnsName,
					},
					LastActive: time.Now().AddDate(0, 0, -idleDays),
				}

				resources = append(resources, resource)
			}
		}
	}

	// Scan CLBs
	clbResp, err := s.client.ELB.DescribeLoadBalancers(ctx, nil)
	if err == nil {
		for _, lb := range clbResp.LoadBalancerDescriptions {
			if lb.LoadBalancerName == nil {
				continue
			}
			if s.isIdleClassicLB(*lb.LoadBalancerName, config.IdleDays) {
				idleDays := s.calculateIdleDays(lb.CreatedTime)

				dnsName := ""
				if lb.DNSName != nil {
					dnsName = *lb.DNSName
				}

				resource := types.Resource{
					ID:          *lb.LoadBalancerName,
					Type:        "loadbalancer",
					Region:      s.client.Config.Region,
					Name:        *lb.LoadBalancerName,
					State:       "classic",
					IdleDays:    idleDays,
					MonthlyCost: s.calc.LoadBalancerCost("classic"),
					Metadata: map[string]string{
						"type":     "classic",
						"dns_name": dnsName,
					},
					LastActive: time.Now().AddDate(0, 0, -idleDays),
				}

				resources = append(resources, resource)
			}
		}
	}

	return resources, nil
}

func (s *LoadBalancerScanner) ResourceType() string {
	return "loadbalancer"
}

func (s *LoadBalancerScanner) Description() string {
	return "Idle load balancers (ALB/NLB/CLB)"
}

func (s *LoadBalancerScanner) isIdleLoadBalancer(arn string, idleDays int) bool {
	// Extract the LB dimension value from the ARN
	// ARN format: arn:aws:elasticloadbalancing:region:account:loadbalancer/app/name/id
	parts := strings.SplitN(arn, ":loadbalancer/", 2)
	if len(parts) < 2 {
		return false
	}
	lbDimension := parts[1]

	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -idleDays)
	period := int32(86400)

	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  strPtr("AWS/ApplicationELB"),
		MetricName: strPtr("RequestCount"),
		Dimensions: []cwtypes.Dimension{
			{Name: strPtr("LoadBalancer"), Value: &lbDimension},
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
		return false
	}

	for _, dp := range resp.Datapoints {
		if dp.Sum != nil && *dp.Sum > 0 {
			return false
		}
	}

	return true
}

func (s *LoadBalancerScanner) isIdleClassicLB(name string, idleDays int) bool {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -idleDays)
	period := int32(86400)

	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  strPtr("AWS/ELB"),
		MetricName: strPtr("RequestCount"),
		Dimensions: []cwtypes.Dimension{
			{Name: strPtr("LoadBalancerName"), Value: &name},
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
		return false
	}

	for _, dp := range resp.Datapoints {
		if dp.Sum != nil && *dp.Sum > 0 {
			return false
		}
	}

	return true
}

func (s *LoadBalancerScanner) calculateIdleDays(createTime *time.Time) int {
	if createTime == nil {
		return 999
	}
	return int(time.Since(*createTime).Hours() / 24)
}
