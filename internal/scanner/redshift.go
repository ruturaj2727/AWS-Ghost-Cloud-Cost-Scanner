package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/cost"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
)

// RedshiftScanner scans for idle Redshift clusters
type RedshiftScanner struct {
	client *aws.Client
	calc   *cost.Calculator
}

// NewRedshiftScanner creates a new Redshift scanner
func NewRedshiftScanner(client *aws.Client) *RedshiftScanner {
	return &RedshiftScanner{
		client: client,
		calc:   cost.NewCalculator(),
	}
}

// Scan returns idle Redshift clusters
func (s *RedshiftScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	var resources []types.Resource

	ctx := context.TODO()

	clusters, err := s.client.Redshift.DescribeClusters(ctx, &redshift.DescribeClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe Redshift clusters: %w", err)
	}

	for _, cluster := range clusters.Clusters {
		if cluster.ClusterIdentifier == nil {
			continue
		}

		// Check for idle clusters
		if s.isIdleCluster(cluster) {
			idleDays := s.calculateIdleDays(cluster)
			cost := s.calculateClusterCost(cluster)

			resource := types.Resource{
				ID:          *cluster.ClusterIdentifier,
				Type:        "Redshift",
				Region:      s.client.Config.Region,
				Name:        getRedshiftClusterName(cluster.ClusterIdentifier, cluster.DBName),
				State:       "Idle",
				IdleDays:    idleDays,
				MonthlyCost: cost,
				Metadata: map[string]string{
					"node_type":     getRedshiftNodeType(cluster),
					"node_count":    getNodeCount(cluster),
					"status":        string(*cluster.ClusterStatus),
					"database_name": getDatabaseName(cluster),
					"encrypted":     fmt.Sprintf("%t", isRedshiftEncrypted(cluster)),
				},
				LastActive: time.Now().AddDate(0, 0, -idleDays),
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func (s *RedshiftScanner) ResourceType() string {
	return "redshift"
}

func (s *RedshiftScanner) Description() string {
	return "Idle Redshift clusters"
}

func (s *RedshiftScanner) isIdleCluster(cluster redshifttypes.Cluster) bool {
	// Consider idle if cluster is available but potentially underutilized
	// In production, you'd check CloudWatch metrics for actual usage
	return cluster.ClusterStatus != nil && *cluster.ClusterStatus == "available"
}

func (s *RedshiftScanner) calculateIdleDays(cluster redshifttypes.Cluster) int {
	// Default to 30 days if we can't determine exact idle time
	// In production, you'd check CloudWatch metrics for actual activity
	return 30
}

func (s *RedshiftScanner) calculateClusterCost(cluster redshifttypes.Cluster) float64 {
	// Estimate cost based on node type and count
	nodeCount := getNodeCountAsInt(cluster)

	// Default to dc2.large pricing estimation
	baseCost := 0.25 * float64(nodeCount) * 730 // $0.25/hr * hours/month

	return baseCost
}

func getRedshiftClusterName(identifier *string, dbName *string) string {
	if identifier != nil && *identifier != "" {
		return *identifier
	}
	if dbName != nil {
		return *dbName
	}
	return "unknown"
}

func getRedshiftNodeType(cluster redshifttypes.Cluster) string {
	if cluster.NodeType != nil {
		return *cluster.NodeType
	}
	return "unknown"
}

func getNodeCount(cluster redshifttypes.Cluster) string {
	if cluster.NumberOfNodes != nil {
		return fmt.Sprintf("%d", *cluster.NumberOfNodes)
	}
	return "1"
}

func getNodeCountAsInt(cluster redshifttypes.Cluster) int {
	if cluster.NumberOfNodes != nil {
		return int(*cluster.NumberOfNodes)
	}
	return 1
}

func getDatabaseName(cluster redshifttypes.Cluster) string {
	if cluster.DBName != nil {
		return *cluster.DBName
	}
	return "unknown"
}

func isRedshiftEncrypted(cluster redshifttypes.Cluster) bool {
	if cluster.Encrypted != nil {
		return *cluster.Encrypted
	}
	return false
}
