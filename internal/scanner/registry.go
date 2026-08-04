package scanner

import (
	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
)

// Registry manages all available scanners
type Registry struct {
	scanners map[string]types.Scanner
}

// NewRegistry creates a new scanner registry
func NewRegistry(client *aws.Client) *Registry {
	r := &Registry{
		scanners: make(map[string]types.Scanner),
	}

	// Register all scanners
	r.Register("ebs", NewEBSScanner(client))
	r.Register("eip", NewEIPScanner(client))
	r.Register("loadbalancer", NewLoadBalancerScanner(client))
	r.Register("nat", NewNATScanner(client))
	r.Register("snapshots", NewSnapshotScanner(client))
	r.Register("ecr", NewECRScanner(client))
	r.Register("s3", NewS3Scanner(client.S3))
	r.Register("cloudfront", NewCloudFrontScanner(client.CloudFront, client.CloudWatch))
	r.Register("autoscaling", NewAutoScalingScanner(client.AutoScaling, client.EC2, client.CloudWatch))
	r.Register("containers", NewContainerScanner(client.ECS, client.EKS, client.CloudWatch))
	r.Register("elasticache", NewElastiCacheScanner(client))
	r.Register("opensearch", NewOpenSearchScanner(client))
	r.Register("redshift", NewRedshiftScanner(client))
	r.Register("dynamodb", NewDynamoDBScanner(client))
	r.Register("kinesis", NewKinesisScanner(client))
	r.Register("sqs", NewSQSScanner(client))
	r.Register("sns", NewSNSScanner(client))

	return r
}

// Register adds a scanner to the registry
func (r *Registry) Register(name string, scanner types.Scanner) {
	r.scanners[name] = scanner
}

// Get returns a scanner by name
func (r *Registry) Get(name string) (types.Scanner, bool) {
	scanner, ok := r.scanners[name]
	return scanner, ok
}

// GetAll returns all registered scanners
func (r *Registry) GetAll() map[string]types.Scanner {
	return r.scanners
}

// GetFiltered returns scanners based on include/exclude lists
func (r *Registry) GetFiltered(only, skip []string) map[string]types.Scanner {
	result := make(map[string]types.Scanner)

	for name, scanner := range r.scanners {
		// Check if in skip list
		if contains(skip, name) {
			continue
		}

		// Check if only list is specified and scanner is not in it
		if len(only) > 0 && !contains(only, name) {
			continue
		}

		result[name] = scanner
	}

	return result
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
