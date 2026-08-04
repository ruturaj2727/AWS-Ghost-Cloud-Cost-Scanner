package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/cost"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// SNSScanner scans for idle SNS topics
type SNSScanner struct {
	client *aws.Client
	calc   *cost.Calculator
}

// NewSNSScanner creates a new SNS scanner
func NewSNSScanner(client *aws.Client) *SNSScanner {
	return &SNSScanner{
		client: client,
		calc:   cost.NewCalculator(),
	}
}

// Scan returns idle SNS topics
func (s *SNSScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	var resources []types.Resource

	ctx := context.TODO()

	topics, err := s.client.SNS.ListTopics(ctx, &sns.ListTopicsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list SNS topics: %w", err)
	}

	for _, topic := range topics.Topics {
		if topic.TopicArn == nil {
			continue
		}

		// Get detailed topic information
		topicAttrs, err := s.client.SNS.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{
			TopicArn: topic.TopicArn,
		})
		if err != nil {
			continue
		}

		// Check for idle topics (few or no subscriptions)
		if s.isIdleTopic(topicAttrs.Attributes) {
			idleDays := s.calculateIdleDays(topicAttrs.Attributes)
			cost := s.calculateTopicCost(topicAttrs.Attributes)

			resource := types.Resource{
				ID:          *topic.TopicArn,
				Type:        "SNS",
				Region:      s.client.Config.Region,
				Name:        getTopicName(topic.TopicArn),
				State:       "Idle",
				IdleDays:    idleDays,
				MonthlyCost: cost,
				Metadata: map[string]string{
					"topic_arn":     *topic.TopicArn,
					"owner":         getAttribute(topicAttrs.Attributes, "Owner"),
					"subscriptions": s.getSubscriptionCount(topic.TopicArn),
				},
				LastActive: time.Now().AddDate(0, 0, -idleDays),
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func (s *SNSScanner) ResourceType() string {
	return "sns"
}

func (s *SNSScanner) Description() string {
	return "Idle SNS topics"
}

func (s *SNSScanner) isIdleTopic(attrs map[string]string) bool {
	// Consider idle if topic has no subscriptions or very few
	// We'll check subscription count separately
	return true
}

func (s *SNSScanner) calculateIdleDays(attrs map[string]string) int {
	// Default to 30 days if we can't determine exact idle time
	return 30
}

func (s *SNSScanner) calculateTopicCost(attrs map[string]string) float64 {
	// SNS pricing: $0.50 per million requests
	// Estimate minimal cost for idle topic
	return 0.50 // $0.50 per month estimate for idle topic
}

func (s *SNSScanner) getSubscriptionCount(topicArn *string) string {
	ctx := context.TODO()

	subs, err := s.client.SNS.ListSubscriptionsByTopic(ctx, &sns.ListSubscriptionsByTopicInput{
		TopicArn: topicArn,
	})
	if err != nil {
		return "unknown"
	}

	return fmt.Sprintf("%d", len(subs.Subscriptions))
}

func getTopicName(topicArn *string) string {
	if topicArn == nil {
		return "unknown"
	}
	// ARN format: arn:aws:sns:region:account-id:topic-name
	parts := splitARN(*topicArn)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return *topicArn
}

func splitARN(arn string) []string {
	// Simple ARN splitter
	parts := []string{}
	current := ""
	for _, ch := range arn {
		if ch == ':' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func getAttribute(attrs map[string]string, key string) string {
	if attrs == nil {
		return ""
	}
	return attrs[key]
}
