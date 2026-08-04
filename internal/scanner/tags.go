package scanner

import (
	"context"

	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// TagFilter provides tag-based filtering for AWS resources
type TagFilter struct {
	ec2Client *ec2.Client
}

// NewTagFilter creates a new tag filter
func NewTagFilter(client interface{}) *TagFilter {
	return &TagFilter{
		ec2Client: client.(*ec2.Client),
	}
}

// FilterResult represents the result of tag filtering
type FilterResult struct {
	ShouldSkip bool
	Reason     string
}

// FilterResource checks if a resource should be skipped based on its tags
func (tf *TagFilter) FilterResource(ctx context.Context, resourceType, resourceID string) FilterResult {
	// Default to not skipping
	result := FilterResult{
		ShouldSkip: false,
		Reason:     "",
	}

	tags, err := tf.getResourceTags(ctx, resourceType, resourceID)
	if err != nil {
		// If we can't get tags, don't skip (be safe)
		return result
	}

	// Check for protection tags
	for _, tag := range tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)

		// Skip resources with keep=true
		if key == "keep" && value == "true" {
			result.ShouldSkip = true
			result.Reason = "Resource has tag keep=true"
			return result
		}

		// Skip production resources
		if key == "env" && (value == "prod" || value == "production") {
			result.ShouldSkip = true
			result.Reason = "Resource has tag env=prod"
			return result
		}

		// Skip resources with do-not-delete=true
		if key == "do-not-delete" && value == "true" {
			result.ShouldSkip = true
			result.Reason = "Resource has tag do-not-delete=true"
			return result
		}

		// Skip resources with backup=true
		if key == "backup" && value == "true" {
			result.ShouldSkip = true
			result.Reason = "Resource has tag backup=true"
			return result
		}

		// Skip resources with critical=true
		if key == "critical" && value == "true" {
			result.ShouldSkip = true
			result.Reason = "Resource has tag critical=true"
			return result
		}
	}

	// Check for owner tags (optional organizational policy)
	if tf.hasOwnerTag(tags) {
		// Don't skip if it has an owner tag, but you could implement
		// logic here to require owner confirmation
	}

	return result
}

// getResourceTags gets tags for a specific resource
func (tf *TagFilter) getResourceTags(ctx context.Context, resourceType, resourceID string) ([]ec2types.Tag, error) {
	switch resourceType {
	case "EBS Volume":
		return tf.getEBSTags(ctx, resourceID)
	case "Elastic IP":
		return tf.getEIPTags(ctx, resourceID)
	case "EC2 Instance":
		return tf.getEC2InstanceTags(ctx, resourceID)
	case "Load Balancer":
		return tf.getLoadBalancerTags(ctx, resourceID)
	case "Auto Scaling Group":
		return tf.getAutoScalingGroupTags(ctx, resourceID)
	default:
		return []ec2types.Tag{}, nil
	}
}

// getEBSTags gets tags for an EBS volume
func (tf *TagFilter) getEBSTags(ctx context.Context, volumeID string) ([]ec2types.Tag, error) {
	result, err := tf.ec2Client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
		VolumeIds: []string{volumeID},
	})
	if err != nil {
		return nil, err
	}

	if len(result.Volumes) == 0 {
		return []ec2types.Tag{}, nil
	}

	return convertTags(result.Volumes[0].Tags), nil
}

// getEIPTags gets tags for an Elastic IP
func (tf *TagFilter) getEIPTags(ctx context.Context, allocationID string) ([]ec2types.Tag, error) {
	result, err := tf.ec2Client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{
		AllocationIds: []string{allocationID},
	})
	if err != nil {
		return nil, err
	}

	if len(result.Addresses) == 0 {
		return []ec2types.Tag{}, nil
	}

	return convertTags(result.Addresses[0].Tags), nil
}

// getEC2InstanceTags gets tags for an EC2 instance
func (tf *TagFilter) getEC2InstanceTags(ctx context.Context, instanceID string) ([]ec2types.Tag, error) {
	result, err := tf.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return nil, err
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return []ec2types.Tag{}, nil
	}

	return convertTags(result.Reservations[0].Instances[0].Tags), nil
}

// getLoadBalancerTags gets tags for a load balancer
func (tf *TagFilter) getLoadBalancerTags(ctx context.Context, lbARN string) ([]ec2types.Tag, error) {
	// This would need to use ELBv2 client for ALB/NLB
	// For now, return empty tags
	return []ec2types.Tag{}, nil
}

// getAutoScalingGroupTags gets tags for an Auto Scaling Group
func (tf *TagFilter) getAutoScalingGroupTags(ctx context.Context, asgName string) ([]ec2types.Tag, error) {
	// This would need to use AutoScaling client
	// For now, return empty tags
	return []ec2types.Tag{}, nil
}

// convertTags converts AWS SDK tags to our tag format
func convertTags(awsTags []ec2types.Tag) []ec2types.Tag {
	var tags []ec2types.Tag
	for _, tag := range awsTags {
		tags = append(tags, ec2types.Tag{
			Key:   aws.String(aws.ToString(tag.Key)),
			Value: aws.String(aws.ToString(tag.Value)),
		})
	}
	return tags
}

// hasOwnerTag checks if resource has an owner tag
func (tf *TagFilter) hasOwnerTag(tags []ec2types.Tag) bool {
	for _, tag := range tags {
		if tag.Key != nil && (*tag.Key == "owner" || *tag.Key == "Owner" || *tag.Key == "created-by") {
			return true
		}
	}
	return false
}

// GroupResourcesByTag groups resources by a specific tag value
func (tf *TagFilter) GroupResourcesByTag(resources []types.Resource, tagKey string) map[string][]types.Resource {
	groups := make(map[string][]types.Resource)

	for _, resource := range resources {
		// Get tags for this resource
		tags, err := tf.getResourceTags(context.Background(), resource.Type, resource.ID)
		if err != nil {
			// Put in "unknown" group if we can't get tags
			groups["unknown"] = append(groups["unknown"], resource)
			continue
		}

		// Find the tag value
		var tagValue string
		for _, tag := range tags {
			if tag.Key != nil && *tag.Key == tagKey {
				tagValue = aws.ToString(tag.Value)
				break
			}
		}

		if tagValue == "" {
			tagValue = "no-tag"
		}

		groups[tagValue] = append(groups[tagValue], resource)
	}

	return groups
}

// GetResourceCostsByTag gets total costs grouped by tag value
func (tf *TagFilter) GetResourceCostsByTag(resources []types.Resource, tagKey string) map[string]float64 {
	costs := make(map[string]float64)

	for _, resource := range resources {
		// Get tags for this resource
		tags, err := tf.getResourceTags(context.Background(), resource.Type, resource.ID)
		if err != nil {
			costs["unknown"] += resource.MonthlyCost
			continue
		}

		// Find the tag value
		var tagValue string
		for _, tag := range tags {
			if tag.Key != nil && *tag.Key == tagKey {
				tagValue = aws.ToString(tag.Value)
				break
			}
		}

		if tagValue == "" {
			tagValue = "no-tag"
		}

		costs[tagValue] += resource.MonthlyCost
	}

	return costs
}

// FilterByTagValue filters resources to only include those with a specific tag value
func (tf *TagFilter) FilterByTagValue(resources []types.Resource, tagKey, tagValue string) []types.Resource {
	var filtered []types.Resource

	for _, resource := range resources {
		tags, err := tf.getResourceTags(context.Background(), resource.Type, resource.ID)
		if err != nil {
			continue
		}

		// Check if resource has the tag with the specified value
		for _, tag := range tags {
			if tag.Key != nil && tag.Value != nil && *tag.Key == tagKey && *tag.Value == tagValue {
				filtered = append(filtered, resource)
				break
			}
		}
	}

	return filtered
}

// ExcludeByTagValue excludes resources that have a specific tag value
func (tf *TagFilter) ExcludeByTagValue(resources []types.Resource, tagKey, tagValue string) []types.Resource {
	var filtered []types.Resource

	for _, resource := range resources {
		tags, err := tf.getResourceTags(context.Background(), resource.Type, resource.ID)
		if err != nil {
			// Include if we can't get tags (be safe)
			filtered = append(filtered, resource)
			continue
		}

		// Check if resource has the tag with the specified value
		shouldExclude := false
		for _, tag := range tags {
			if tag.Key != nil && tag.Value != nil && *tag.Key == tagKey && *tag.Value == tagValue {
				shouldExclude = true
				break
			}
		}

		if !shouldExclude {
			filtered = append(filtered, resource)
		}
	}

	return filtered
}
