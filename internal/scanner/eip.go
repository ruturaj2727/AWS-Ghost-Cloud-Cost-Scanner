package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/cost"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
)

// EIPScanner scans for unattached Elastic IPs
type EIPScanner struct {
	client *aws.Client
	calc   *cost.Calculator
}

// NewEIPScanner creates a new EIP scanner
func NewEIPScanner(client *aws.Client) *EIPScanner {
	return &EIPScanner{
		client: client,
		calc:   cost.NewCalculator(),
	}
}

// Scan returns unattached Elastic IPs
func (s *EIPScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	var resources []types.Resource

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resp, err := s.client.EC2.DescribeAddresses(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to describe Elastic IPs: %w\nTip: Ensure you have ec2:DescribeAddresses permission", err)
	}

	for _, addr := range resp.Addresses {
		// Skip if essential fields are nil
		if addr.AllocationId == nil || addr.PublicIp == nil {
			continue
		}

		// Check if IP is not associated with a running instance
		if addr.AssociationId == nil || addr.InstanceId == nil {
			// Use a default idle time since AllocationTime is not available
			idleDays := 30

			publicIP := *addr.PublicIp
			resource := types.Resource{
				ID:          *addr.AllocationId,
				Type:        "eip",
				Region:      s.client.Config.Region,
				Name:        publicIP,
				State:       "unattached",
				IdleDays:    idleDays,
				MonthlyCost: s.calc.ElasticIPCost(),
				Metadata: map[string]string{
					"public_ip": publicIP,
					"domain":    string(addr.Domain),
				},
				LastActive: time.Now().AddDate(0, 0, -idleDays),
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func (s *EIPScanner) ResourceType() string {
	return "eip"
}

func (s *EIPScanner) Description() string {
	return "Unattached Elastic IPs"
}
