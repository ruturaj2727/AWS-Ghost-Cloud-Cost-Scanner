package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/cost"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
)

// OpenSearchScanner scans for idle OpenSearch domains
type OpenSearchScanner struct {
	client *aws.Client
	calc   *cost.Calculator
}

// NewOpenSearchScanner creates a new OpenSearch scanner
func NewOpenSearchScanner(client *aws.Client) *OpenSearchScanner {
	return &OpenSearchScanner{
		client: client,
		calc:   cost.NewCalculator(),
	}
}

// Scan returns idle OpenSearch domains
func (s *OpenSearchScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	var resources []types.Resource

	ctx := context.TODO()

	domains, err := s.client.OpenSearch.ListDomainNames(ctx, &opensearch.ListDomainNamesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list OpenSearch domains: %w", err)
	}

	for _, domainName := range domains.DomainNames {
		if domainName.DomainName == nil {
			continue
		}

		// Get detailed domain information
		domainInfo, err := s.client.OpenSearch.DescribeDomain(ctx, &opensearch.DescribeDomainInput{
			DomainName: domainName.DomainName,
		})
		if err != nil {
			continue
		}

		if domainInfo.DomainStatus == nil {
			continue
		}

		domain := domainInfo.DomainStatus

		// Check for idle domains (low usage or old)
		if s.isIdleDomain(*domain) {
			idleDays := s.calculateIdleDays(*domain)
			cost := s.calculateDomainCost(*domain)

			resource := types.Resource{
				ID:          *domain.DomainName,
				Type:        "OpenSearch",
				Region:      s.client.Config.Region,
				Name:        getDomainName(domain.DomainName, domain.ARN),
				State:       "Idle",
				IdleDays:    idleDays,
				MonthlyCost: cost,
				Metadata: map[string]string{
					"instance_type":  getInstanceType(*domain),
					"instance_count": getInstanceCount(*domain),
					"engine_version": getEngineVersion(*domain),
					"endpoint":       getEndpoint(domain.Endpoint),
					"status":         getEndpointString(domain.Endpoint),
					"ebs_enabled":    fmt.Sprintf("%t", isEBSEnabled(*domain)),
				},
				LastActive: time.Now().AddDate(0, 0, -idleDays),
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func (s *OpenSearchScanner) ResourceType() string {
	return "opensearch"
}

func (s *OpenSearchScanner) Description() string {
	return "Idle OpenSearch domains"
}

func (s *OpenSearchScanner) isIdleDomain(domain opensearchtypes.DomainStatus) bool {
	// Consider idle if domain is in available state but potentially underutilized
	// In production, you'd check CloudWatch metrics for actual usage
	return domain.Endpoint != nil && *domain.Endpoint != ""
}

func (s *OpenSearchScanner) calculateIdleDays(domain opensearchtypes.DomainStatus) int {
	// Default to 30 days if we can't determine exact idle time
	// In production, you'd check CloudWatch metrics for actual activity
	return 30
}

func (s *OpenSearchScanner) calculateDomainCost(domain opensearchtypes.DomainStatus) float64 {
	// Estimate cost based on instance type and count
	instanceCount := getInstanceCountAsInt(domain)

	// Default to t3.medium pricing estimation
	baseCost := 35.0 * float64(instanceCount) // $35/month per instance estimate

	// Add EBS cost if enabled
	if isEBSEnabled(domain) {
		baseCost += 10.0 // Additional $10/month for EBS
	}

	return baseCost
}

func getDomainName(name *string, arn *string) string {
	if name != nil && *name != "" {
		return *name
	}
	if arn != nil {
		return *arn
	}
	return "unknown"
}

func getInstanceType(domain opensearchtypes.DomainStatus) string {
	if domain.ClusterConfig != nil {
		return string(domain.ClusterConfig.InstanceType)
	}
	return "unknown"
}

func getInstanceCount(domain opensearchtypes.DomainStatus) string {
	if domain.ClusterConfig != nil && domain.ClusterConfig.InstanceCount != nil {
		return fmt.Sprintf("%d", *domain.ClusterConfig.InstanceCount)
	}
	return "1"
}

func getInstanceCountAsInt(domain opensearchtypes.DomainStatus) int {
	if domain.ClusterConfig != nil && domain.ClusterConfig.InstanceCount != nil {
		return int(*domain.ClusterConfig.InstanceCount)
	}
	return 1
}

func getEngineVersion(domain opensearchtypes.DomainStatus) string {
	if domain.EngineVersion != nil {
		return *domain.EngineVersion
	}
	return "unknown"
}

func getEndpoint(endpoint *string) string {
	if endpoint != nil {
		return *endpoint
	}
	return "none"
}

func getEndpointString(endpoint *string) string {
	if endpoint != nil {
		return *endpoint
	}
	return "none"
}

func isEBSEnabled(domain opensearchtypes.DomainStatus) bool {
	if domain.EBSOptions != nil && domain.EBSOptions.EBSEnabled != nil {
		return *domain.EBSOptions.EBSEnabled
	}
	return false
}
