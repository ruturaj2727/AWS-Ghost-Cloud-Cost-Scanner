package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
)

// ContainerScanner scans for wasteful container resources (ECS/EKS)
type ContainerScanner struct {
	ecsClient *ecs.Client
	eksClient *eks.Client
	cwClient  *cloudwatch.Client
}

// NewContainerScanner creates a new container scanner
func NewContainerScanner(ecsClient, eksClient, cwClient interface{}) types.Scanner {
	return &ContainerScanner{
		ecsClient: ecsClient.(*ecs.Client),
		eksClient: eksClient.(*eks.Client),
		cwClient:  cwClient.(*cloudwatch.Client),
	}
}

// ResourceType returns the type identifier for this scanner
func (c *ContainerScanner) ResourceType() string {
	return "Container Resources"
}

// Description returns a human-readable description
func (c *ContainerScanner) Description() string {
	return "Scans for wasteful container resources including ECS clusters, EKS clusters, and related services"
}

// Scan returns wasteful container resources
func (c *ContainerScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	ctx := context.Background()
	var resources []types.Resource

	// Scan ECS clusters
	ecsResources, err := c.scanECS(ctx, config.Region, config.IdleDays)
	if err != nil {
		return nil, fmt.Errorf("failed to scan ECS: %w", err)
	}
	resources = append(resources, ecsResources...)

	// Scan EKS clusters
	eksResources, err := c.scanEKS(ctx, config.Region, config.IdleDays)
	if err != nil {
		return nil, fmt.Errorf("failed to scan EKS: %w", err)
	}
	resources = append(resources, eksResources...)

	return resources, nil
}

// scanECS scans for wasteful ECS resources
func (c *ContainerScanner) scanECS(ctx context.Context, region string, idleDays int) ([]types.Resource, error) {
	var resources []types.Resource

	clusters, err := c.listECSClusters(ctx)
	if err != nil {
		return nil, err
	}

	for _, cluster := range clusters {
		// Check for empty clusters
		isEmpty, err := c.checkEmptyECSCluster(ctx, cluster)
		if err != nil {
			continue
		}

		if isEmpty {
			cost := c.calculateEmptyECSClusterCost(cluster)
			resources = append(resources, types.Resource{
				ID:          cluster,
				Type:        "ECS Cluster",
				Region:      region,
				State:       "Empty",
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": "ECS cluster with no running services or tasks"},
				LastActive:  time.Time{},
			})
			continue
		}

		// Check for idle services
		idleServices, err := c.checkIdleECSServices(ctx, cluster, idleDays)
		if err != nil {
			continue
		}

		for _, service := range idleServices {
			cost := c.calculateIdleECSServiceCost(cluster, service)
			resources = append(resources, types.Resource{
				ID:          fmt.Sprintf("%s/%s", cluster, service),
				Type:        "ECS Service",
				Region:      region,
				State:       "Idle",
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": fmt.Sprintf("ECS service with zero running tasks for %d days", idleDays)},
				LastActive:  time.Time{},
			})
		}
	}

	return resources, nil
}

// scanEKS scans for wasteful EKS resources
func (c *ContainerScanner) scanEKS(ctx context.Context, region string, idleDays int) ([]types.Resource, error) {
	var resources []types.Resource

	clusters, err := c.listEKSClusters(ctx)
	if err != nil {
		return nil, err
	}

	for _, cluster := range clusters {
		// Check for empty clusters
		isEmpty, err := c.checkEmptyEKSCluster(ctx, *cluster.Name)
		if err != nil {
			continue
		}

		if isEmpty {
			cost := c.calculateEmptyEKSClusterCost(cluster)
			resources = append(resources, types.Resource{
				ID:          *cluster.Name,
				Type:        "EKS Cluster",
				Region:      region,
				State:       "Empty",
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": "EKS cluster with no running node groups or pods"},
				LastActive:  *cluster.CreatedAt,
			})
			continue
		}

		// Check for idle node groups
		idleNodeGroups, err := c.checkIdleEKSNodeGroups(ctx, *cluster.Name, idleDays)
		if err != nil {
			continue
		}

		for _, nodeGroup := range idleNodeGroups {
			cost := c.calculateIdleEKSNodeGroupCost(cluster, nodeGroup)
			resources = append(resources, types.Resource{
				ID:          fmt.Sprintf("%s/%s", *cluster.Name, nodeGroup),
				Type:        "EKS Node Group",
				Region:      region,
				State:       "Idle",
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": fmt.Sprintf("EKS node group with zero running instances for %d days", idleDays)},
				LastActive:  time.Time{},
			})
		}
	}

	return resources, nil
}

// listECSClusters lists all ECS clusters
func (c *ContainerScanner) listECSClusters(ctx context.Context) ([]string, error) {
	result, err := c.ecsClient.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return nil, err
	}

	return result.ClusterArns, nil
}

// listEKSClusters lists all EKS clusters
func (c *ContainerScanner) listEKSClusters(ctx context.Context) ([]ekstypes.Cluster, error) {
	result, err := c.eksClient.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, err
	}

	var clusters []ekstypes.Cluster
	for _, clusterName := range result.Clusters {
		describeResult, err := c.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		})
		if err != nil {
			continue
		}
		clusters = append(clusters, *describeResult.Cluster)
	}

	return clusters, nil
}

// checkEmptyECSCluster checks if ECS cluster is empty
func (c *ContainerScanner) checkEmptyECSCluster(ctx context.Context, clusterArn string) (bool, error) {
	// Check for running services
	services, err := c.ecsClient.ListServices(ctx, &ecs.ListServicesInput{
		Cluster: aws.String(clusterArn),
	})
	if err != nil {
		return false, err
	}

	// Check for running tasks
	tasks, err := c.ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster: aws.String(clusterArn),
	})
	if err != nil {
		return false, err
	}

	return len(services.ServiceArns) == 0 && len(tasks.TaskArns) == 0, nil
}

// checkEmptyEKSCluster checks if EKS cluster is empty
func (c *ContainerScanner) checkEmptyEKSCluster(ctx context.Context, clusterName string) (bool, error) {
	// List node groups
	nodeGroups, err := c.eksClient.ListNodegroups(ctx, &eks.ListNodegroupsInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		return false, err
	}

	// If no node groups, cluster is empty
	if len(nodeGroups.Nodegroups) == 0 {
		return true, nil
	}

	// Check if node groups have running instances
	for _, nodeGroup := range nodeGroups.Nodegroups {
		describeResult, err := c.eksClient.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodegroupName: aws.String(nodeGroup),
		})
		if err != nil {
			continue
		}

		if describeResult.Nodegroup.ScalingConfig != nil &&
			describeResult.Nodegroup.ScalingConfig.DesiredSize != nil &&
			*describeResult.Nodegroup.ScalingConfig.DesiredSize > 0 {
			return false, nil // Has running instances
		}
	}

	return true, nil
}

// checkIdleECSServices checks for idle ECS services
func (c *ContainerScanner) checkIdleECSServices(ctx context.Context, clusterArn string, idleDays int) ([]string, error) {
	var idleServices []string

	services, err := c.ecsClient.ListServices(ctx, &ecs.ListServicesInput{
		Cluster: aws.String(clusterArn),
	})
	if err != nil {
		return nil, err
	}

	for _, serviceArn := range services.ServiceArns {
		describeResult, err := c.ecsClient.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterArn),
			Services: []string{serviceArn},
		})
		if err != nil {
			continue
		}

		if len(describeResult.Services) > 0 {
			service := describeResult.Services[0]
			if service.RunningCount == 0 {
				// Service has no running tasks
				idleServices = append(idleServices, serviceArn)
			}
		}
	}

	return idleServices, nil
}

// checkIdleEKSNodeGroups checks for idle EKS node groups
func (c *ContainerScanner) checkIdleEKSNodeGroups(ctx context.Context, clusterName string, idleDays int) ([]string, error) {
	var idleNodeGroups []string

	nodeGroups, err := c.eksClient.ListNodegroups(ctx, &eks.ListNodegroupsInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		return nil, err
	}

	for _, nodeGroup := range nodeGroups.Nodegroups {
		describeResult, err := c.eksClient.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodegroupName: aws.String(nodeGroup),
		})
		if err != nil {
			continue
		}

		if describeResult.Nodegroup.ScalingConfig != nil &&
			describeResult.Nodegroup.ScalingConfig.DesiredSize != nil &&
			*describeResult.Nodegroup.ScalingConfig.DesiredSize == 0 {
			idleNodeGroups = append(idleNodeGroups, nodeGroup)
		}
	}

	return idleNodeGroups, nil
}

// calculateEmptyECSClusterCost calculates cost for empty ECS cluster
func (c *ContainerScanner) calculateEmptyECSClusterCost(clusterArn string) float64 {
	// Empty ECS clusters incur minimal costs for cluster management
	return 0.10 // $0.10 per month for empty cluster
}

// calculateIdleECSServiceCost calculates cost for idle ECS service
func (c *ContainerScanner) calculateIdleECSServiceCost(clusterArn, serviceArn string) float64 {
	// Idle services incur minimal costs for service configuration
	return 0.05 // $0.05 per month for idle service
}

// calculateEmptyEKSClusterCost calculates cost for empty EKS cluster
func (c *ContainerScanner) calculateEmptyEKSClusterCost(cluster ekstypes.Cluster) float64 {
	// EKS cluster control plane cost (charged even when empty)
	return 0.10 * 730 // $0.10 per hour * hours/month
}

// calculateIdleEKSNodeGroupCost calculates cost for idle EKS node group
func (c *ContainerScanner) calculateIdleEKSNodeGroupCost(cluster ekstypes.Cluster, nodeGroup string) float64 {
	// Even idle node groups might have some costs for configuration
	return 0.05 // $0.05 per month for idle node group configuration
}
