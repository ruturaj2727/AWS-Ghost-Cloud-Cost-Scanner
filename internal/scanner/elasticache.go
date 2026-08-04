package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/cost"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
)

// ElastiCacheScanner scans for idle ElastiCache clusters
type ElastiCacheScanner struct {
	client *aws.Client
	calc   *cost.Calculator
}

// NewElastiCacheScanner creates a new ElastiCache scanner
func NewElastiCacheScanner(client *aws.Client) *ElastiCacheScanner {
	return &ElastiCacheScanner{
		client: client,
		calc:   cost.NewCalculator(),
	}
}

// Scan returns idle ElastiCache clusters
func (s *ElastiCacheScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	var resources []types.Resource

	ctx := context.TODO()

	// Scan for Redis clusters
	redisClusters, err := s.client.ElastiCache.DescribeReplicationGroups(ctx, &elasticache.DescribeReplicationGroupsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Redis replication groups: %w", err)
	}

	for _, cluster := range redisClusters.ReplicationGroups {
		if cluster.ReplicationGroupId == nil {
			continue
		}

		// Check for idle clusters (low connection count or no activity)
		if s.isIdleCluster(cluster) {
			idleDays := s.calculateIdleDays(cluster.MemberClusters)
			cost := s.calculateClusterCost(cluster)

			resource := types.Resource{
				ID:          *cluster.ReplicationGroupId,
				Type:        "ElastiCache Redis",
				Region:      s.client.Config.Region,
				Name:        getElastiCacheClusterName(cluster.Description),
				State:       "Idle",
				IdleDays:    idleDays,
				MonthlyCost: cost,
				Metadata: map[string]string{
					"node_type":   getElastiCacheNodeType(cluster.MemberClusters),
					"num_nodes":   fmt.Sprintf("%d", getNumNodes(cluster.MemberClusters)),
					"engine":      "redis",
					"status":      string(*cluster.Status),
					"description": getElastiCacheClusterDescription(cluster.Description),
				},
				LastActive: time.Now().AddDate(0, 0, -idleDays),
			}

			resources = append(resources, resource)
		}
	}

	// Scan for Memcached clusters
	memcachedClusters, err := s.client.ElastiCache.DescribeCacheClusters(ctx, &elasticache.DescribeCacheClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Memcached clusters: %w", err)
	}

	for _, cluster := range memcachedClusters.CacheClusters {
		if cluster.CacheClusterId == nil {
			continue
		}

		// Skip Redis clusters (already handled above)
		if cluster.Engine != nil && *cluster.Engine == "redis" {
			continue
		}

		// Check for idle clusters
		if s.isIdleMemcachedCluster(cluster) {
			idleDays := s.calculateMemcachedIdleDays(cluster)
			cost := s.calculateMemcachedCost(cluster)

			resource := types.Resource{
				ID:          *cluster.CacheClusterId,
				Type:        "ElastiCache Memcached",
				Region:      s.client.Config.Region,
				Name:        getMemcachedClusterName(cluster),
				State:       "Idle",
				IdleDays:    idleDays,
				MonthlyCost: cost,
				Metadata: map[string]string{
					"node_type":      getCacheNodeType(cluster),
					"num_nodes":      fmt.Sprintf("%d", getNumCacheNodes(cluster)),
					"engine":         getCacheEngine(cluster),
					"status":         string(*cluster.CacheClusterStatus),
					"engine_version": getCacheEngineVersion(cluster),
				},
				LastActive: time.Now().AddDate(0, 0, -idleDays),
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func (s *ElastiCacheScanner) ResourceType() string {
	return "elasticache"
}

func (s *ElastiCacheScanner) Description() string {
	return "Idle ElastiCache clusters (Redis and Memcached)"
}

func (s *ElastiCacheScanner) isIdleCluster(cluster elasticachetypes.ReplicationGroup) bool {
	// Consider idle if status is 'available' but has no recent activity indicators
	// This is a simplified check - in production you'd check CloudWatch metrics
	return cluster.Status != nil && *cluster.Status == "available"
}

func (s *ElastiCacheScanner) isIdleMemcachedCluster(cluster elasticachetypes.CacheCluster) bool {
	// Consider idle if status is 'available'
	return cluster.CacheClusterStatus != nil && *cluster.CacheClusterStatus == "available"
}

func (s *ElastiCacheScanner) calculateIdleDays(memberClusters []string) int {
	// Default to 30 days if we can't determine exact idle time
	// In production, you'd check CloudWatch metrics for actual activity
	return 30
}

func (s *ElastiCacheScanner) calculateMemcachedIdleDays(cluster elasticachetypes.CacheCluster) int {
	// Default to 30 days
	return 30
}

func (s *ElastiCacheScanner) calculateClusterCost(cluster elasticachetypes.ReplicationGroup) float64 {
	// Estimate cost based on node type and number of nodes
	// Default to cache.m5.large pricing estimation
	nodeCount := getNumNodes(cluster.MemberClusters)
	return 25.0 * float64(nodeCount) // $25/month per node estimate
}

func (s *ElastiCacheScanner) calculateMemcachedCost(cluster elasticachetypes.CacheCluster) float64 {
	// Estimate cost based on node type and number of nodes
	nodeCount := getNumCacheNodes(cluster)
	return 25.0 * float64(nodeCount) // $25/month per node estimate
}

func getElastiCacheClusterName(desc *string) string {
	if desc != nil && *desc != "" {
		return *desc
	}
	return "unknown"
}

func getElastiCacheClusterDescription(desc *string) string {
	if desc != nil {
		return *desc
	}
	return ""
}

func getElastiCacheNodeType(memberClusters []string) string {
	if len(memberClusters) > 0 {
		return "unknown" // Would need DescribeCacheClusters to get actual node type
	}
	return "unknown"
}

func getNumNodes(memberClusters []string) int {
	return len(memberClusters)
}

func getMemcachedClusterName(cluster elasticachetypes.CacheCluster) string {
	if cluster.CacheClusterId != nil {
		return *cluster.CacheClusterId
	}
	return "unknown"
}

func getCacheNodeType(cluster elasticachetypes.CacheCluster) string {
	if cluster.CacheNodeType != nil {
		return *cluster.CacheNodeType
	}
	return "unknown"
}

func getNumCacheNodes(cluster elasticachetypes.CacheCluster) int {
	if cluster.NumCacheNodes != nil {
		return int(*cluster.NumCacheNodes)
	}
	return 1
}

func getCacheEngine(cluster elasticachetypes.CacheCluster) string {
	if cluster.Engine != nil {
		return *cluster.Engine
	}
	return "unknown"
}

func getCacheEngineVersion(cluster elasticachetypes.CacheCluster) string {
	if cluster.EngineVersion != nil {
		return *cluster.EngineVersion
	}
	return "unknown"
}
