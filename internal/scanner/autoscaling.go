package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// AutoScalingScanner scans for wasteful Auto Scaling Groups
type AutoScalingScanner struct {
	asgClient *autoscaling.Client
	ec2Client *ec2.Client
	cwClient  *cloudwatch.Client
}

// NewAutoScalingScanner creates a new Auto Scaling scanner
func NewAutoScalingScanner(asgClient, ec2Client, cwClient interface{}) types.Scanner {
	return &AutoScalingScanner{
		asgClient: asgClient.(*autoscaling.Client),
		ec2Client: ec2Client.(*ec2.Client),
		cwClient:  cwClient.(*cloudwatch.Client),
	}
}

// ResourceType returns the type identifier for this scanner
func (a *AutoScalingScanner) ResourceType() string {
	return "Auto Scaling Group"
}

// Description returns a human-readable description
func (a *AutoScalingScanner) Description() string {
	return "Scans for wasteful Auto Scaling Groups including empty groups, idle instances, and underutilized resources"
}

// Scan returns wasteful Auto Scaling resources
func (a *AutoScalingScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	ctx := context.Background() // Or get from a global context
	var resources []types.Resource

	asgs, err := a.listAutoScalingGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list Auto Scaling Groups: %w", err)
	}

	for _, asg := range asgs {
		// Check for empty ASGs (0 desired, 0 min, 0 max)
		if a.isEmptyASG(asg) {
			cost := a.calculateEmptyASGCost(asg)
			resources = append(resources, types.Resource{
				ID:          *asg.AutoScalingGroupName,
				Type:        "Auto Scaling Group",
				Region:      config.Region,
				State:       "Empty",
				IdleDays:    0,
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": "ASG configured with 0 min/desired/max capacity"},
				LastActive:  *asg.CreatedTime,
			})
			continue
		}

		// Check for ASGs with no instances running
		if a.hasNoInstances(asg) {
			cost := a.calculateIdleASGCost(asg)
			resources = append(resources, types.Resource{
				ID:          *asg.AutoScalingGroupName,
				Type:        "Auto Scaling Group",
				Region:      config.Region,
				State:       "No instances",
				IdleDays:    0,
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": "ASG has 0 running instances but is still configured"},
				LastActive:  *asg.CreatedTime,
			})
			continue
		}

		// Check for ASGs with consistently low CPU utilization
		hasLowCPU, err := a.checkLowCPUUtilization(ctx, asg, config.IdleDays)
		if err != nil {
			continue
		}

		if hasLowCPU {
			cost := a.calculateUnderutilizedASGCost(asg)
			resources = append(resources, types.Resource{
				ID:          *asg.AutoScalingGroupName,
				Type:        "Auto Scaling Group",
				Region:      config.Region,
				State:       "Underutilized",
				IdleDays:    config.IdleDays,
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": fmt.Sprintf("Average CPU utilization < 10%% for %d days", config.IdleDays)},
				LastActive:  *asg.CreatedTime,
			})
		}

		// Check for ASGs with old launch configurations
		if a.hasOldLaunchConfiguration(asg) {
			cost := a.calculateIdleASGCost(asg) * 0.2 // Additional cost for outdated config
			resources = append(resources, types.Resource{
				ID:          *asg.AutoScalingGroupName,
				Type:        "Auto Scaling Group",
				Region:      config.Region,
				State:       "Old launch config",
				IdleDays:    0,
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": "Using deprecated launch configuration instead of launch template"},
				LastActive:  *asg.CreatedTime,
			})
		}

		// Check for ASGs with scaling policies that never trigger
		if err := a.checkUnusedScalingPolicies(ctx, asg, config.IdleDays); err == nil {
			cost := a.calculateIdleASGCost(asg) * 0.1
			resources = append(resources, types.Resource{
				ID:          *asg.AutoScalingGroupName,
				Type:        "Auto Scaling Group",
				Region:      config.Region,
				State:       "Unused scaling policies",
				IdleDays:    0,
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": "Scaling policies configured but never triggered"},
				LastActive:  *asg.CreatedTime,
			})
		}
	}

	return resources, nil
}

// listAutoScalingGroups lists all Auto Scaling Groups
func (a *AutoScalingScanner) listAutoScalingGroups(ctx context.Context) ([]autoscalingtypes.AutoScalingGroup, error) {
	var asgs []autoscalingtypes.AutoScalingGroup

	result, err := a.asgClient.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		return nil, err
	}

	asgs = append(asgs, result.AutoScalingGroups...)

	return asgs, nil
}

// isEmptyASG checks if ASG is configured with zero capacity
func (a *AutoScalingScanner) isEmptyASG(asg autoscalingtypes.AutoScalingGroup) bool {
	return asg.MinSize != nil && *asg.MinSize == 0 &&
		asg.MaxSize != nil && *asg.MaxSize == 0 &&
		asg.DesiredCapacity != nil && *asg.DesiredCapacity == 0
}

// hasNoInstances checks if ASG has no running instances
func (a *AutoScalingScanner) hasNoInstances(asg autoscalingtypes.AutoScalingGroup) bool {
	return len(asg.Instances) == 0
}

// checkLowCPUUtilization checks if ASG instances have consistently low CPU
func (a *AutoScalingScanner) checkLowCPUUtilization(ctx context.Context, asg autoscalingtypes.AutoScalingGroup, idleDays int) (bool, error) {
	if len(asg.Instances) == 0 {
		return false, nil
	}

	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -idleDays)

	// Get CPU utilization for the ASG
	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/EC2"),
		MetricName: aws.String("CPUUtilization"),
		Dimensions: []cloudwatchtypes.Dimension{
			{
				Name:  aws.String("AutoScalingGroupName"),
				Value: asg.AutoScalingGroupName,
			},
		},
		StartTime:  &startTime,
		EndTime:    &endTime,
		Period:     aws.Int32(3600), // 1 hour
		Statistics: []cloudwatchtypes.Statistic{cloudwatchtypes.StatisticAverage},
	}

	result, err := a.cwClient.GetMetricStatistics(ctx, input)
	if err != nil {
		return false, err
	}

	if len(result.Datapoints) == 0 {
		return false, nil
	}

	// Calculate average CPU utilization
	var totalUtilization float64
	for _, dp := range result.Datapoints {
		if dp.Average != nil {
			totalUtilization += *dp.Average
		}
	}

	avgUtilization := totalUtilization / float64(len(result.Datapoints))
	return avgUtilization < 10.0, nil
}

// hasOldLaunchConfiguration checks if ASG uses deprecated launch configuration
func (a *AutoScalingScanner) hasOldLaunchConfiguration(asg autoscalingtypes.AutoScalingGroup) bool {
	return asg.LaunchConfigurationName != nil && *asg.LaunchConfigurationName != ""
}

// checkUnusedScalingPolicies checks if scaling policies are never used
func (a *AutoScalingScanner) checkUnusedScalingPolicies(ctx context.Context, asg autoscalingtypes.AutoScalingGroup, idleDays int) error {
	// TODO: Implement scaling policies check with correct AWS SDK v2 API
	// For now, return error to skip this check
	return fmt.Errorf("scaling policies check not yet implemented")
}

// calculateEmptyASGCost calculates cost for empty ASG
func (a *AutoScalingScanner) calculateEmptyASGCost(asg autoscalingtypes.AutoScalingGroup) float64 {
	// Empty ASGs incur minimal costs for configuration storage
	return 0.10 // $0.10 per month for empty ASG configuration
}

// calculateIdleASGCost calculates cost for idle ASG
func (a *AutoScalingScanner) calculateIdleASGCost(asg autoscalingtypes.AutoScalingGroup) float64 {
	cost := 0.10 // Base configuration cost

	// Add cost for configured but unused resources
	if asg.DesiredCapacity != nil && *asg.DesiredCapacity > 0 {
		// Estimate instance type cost (assuming t3.micro for estimation)
		instanceCost := 0.0104 * float64(*asg.DesiredCapacity) * 730 // $0.0104/hr * hours/month
		cost += instanceCost
	}

	// Add cost for load balancers if attached
	if len(asg.LoadBalancerNames) > 0 {
		cost += float64(len(asg.LoadBalancerNames)) * 0.0225 * 730 // ALB cost
	}

	// Add cost for target groups
	if len(asg.TargetGroupARNs) > 0 {
		cost += float64(len(asg.TargetGroupARNs)) * 0.0036 * 730 // Target group cost
	}

	return cost
}

// calculateUnderutilizedASGCost calculates cost for underutilized ASG
func (a *AutoScalingScanner) calculateUnderutilizedASGCost(asg autoscalingtypes.AutoScalingGroup) float64 {
	// Full cost since instances are running but underutilized
	return a.calculateIdleASGCost(asg)
}
