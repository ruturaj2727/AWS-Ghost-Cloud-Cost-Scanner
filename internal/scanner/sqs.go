package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/cost"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// SQSScanner scans for idle SQS queues
type SQSScanner struct {
	client *aws.Client
	calc   *cost.Calculator
}

// NewSQSScanner creates a new SQS scanner
func NewSQSScanner(client *aws.Client) *SQSScanner {
	return &SQSScanner{
		client: client,
		calc:   cost.NewCalculator(),
	}
}

// Scan returns idle SQS queues
func (s *SQSScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	var resources []types.Resource

	ctx := context.TODO()

	queues, err := s.client.SQS.ListQueues(ctx, &sqs.ListQueuesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list SQS queues: %w", err)
	}

	for _, queueURL := range queues.QueueUrls {
		// Get detailed queue information
		queueAttrs, err := s.client.SQS.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl: &queueURL,
			AttributeNames: []sqstypes.QueueAttributeName{
				sqstypes.QueueAttributeNameApproximateNumberOfMessages,
				sqstypes.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
				sqstypes.QueueAttributeNameApproximateNumberOfMessagesDelayed,
				sqstypes.QueueAttributeNameCreatedTimestamp,
				sqstypes.QueueAttributeNameLastModifiedTimestamp,
				sqstypes.QueueAttributeNameQueueArn,
			},
		})
		if err != nil {
			continue
		}

		// Check for idle queues (low message count or old)
		if s.isIdleQueue(queueAttrs.Attributes) {
			idleDays := s.calculateIdleDays(queueAttrs.Attributes)
			cost := s.calculateQueueCost(queueAttrs.Attributes)

			resource := types.Resource{
				ID:          queueURL,
				Type:        "SQS",
				Region:      s.client.Config.Region,
				Name:        getQueueNameFromURL(queueURL),
				State:       "Idle",
				IdleDays:    idleDays,
				MonthlyCost: cost,
				Metadata: map[string]string{
					"available_messages": getMessageCount(queueAttrs.Attributes, sqstypes.QueueAttributeNameApproximateNumberOfMessages),
					"in_flight_messages": getMessageCount(queueAttrs.Attributes, sqstypes.QueueAttributeNameApproximateNumberOfMessagesNotVisible),
					"delayed_messages":   getMessageCount(queueAttrs.Attributes, sqstypes.QueueAttributeNameApproximateNumberOfMessagesDelayed),
					"queue_arn":          getSQSAttribute(queueAttrs.Attributes, sqstypes.QueueAttributeNameQueueArn),
					"last_modified":      getSQSAttribute(queueAttrs.Attributes, sqstypes.QueueAttributeNameLastModifiedTimestamp),
				},
				LastActive: time.Now().AddDate(0, 0, -idleDays),
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func (s *SQSScanner) ResourceType() string {
	return "sqs"
}

func (s *SQSScanner) Description() string {
	return "Idle SQS queues"
}

func (s *SQSScanner) isIdleQueue(attrs map[string]string) bool {
	// Consider idle if queue has very few messages
	availableMessages := getSQSAttribute(attrs, sqstypes.QueueAttributeNameApproximateNumberOfMessages)
	if availableMessages == "" {
		return false
	}
	var count int64
	fmt.Sscanf(availableMessages, "%d", &count)
	return count < 10
}

func (s *SQSScanner) calculateIdleDays(attrs map[string]string) int {
	// Try to calculate based on last modified timestamp
	lastModified := getSQSAttribute(attrs, sqstypes.QueueAttributeNameLastModifiedTimestamp)
	if lastModified != "" {
		// Parse timestamp and calculate days
		// For simplicity, default to 30 days
		return 30
	}
	return 30
}

func (s *SQSScanner) calculateQueueCost(attrs map[string]string) float64 {
	// SQS pricing: $0.40 per million requests (first 64KB)
	// Estimate based on idle time - minimal cost for idle queues
	return 0.50 // $0.50 per month estimate for idle queue
}

func getQueueNameFromURL(queueURL string) string {
	// Extract queue name from URL
	// URL format: https://queue.amazonaws.com/account-id/queue-name
	parts := splitURL(queueURL)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return queueURL
}

func splitURL(url string) []string {
	// Simple URL splitter
	parts := []string{}
	current := ""
	for _, ch := range url {
		if ch == '/' {
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

func getMessageCount(attrs map[string]string, attrName sqstypes.QueueAttributeName) string {
	return getSQSAttribute(attrs, attrName)
}

func getMessageCountAsInt(attrs map[string]string, attrName sqstypes.QueueAttributeName) int64 {
	value := getSQSAttribute(attrs, attrName)
	if value == "" {
		return 0
	}
	var count int64
	fmt.Sscanf(value, "%d", &count)
	return count
}

func getSQSAttribute(attrs map[string]string, attrName sqstypes.QueueAttributeName) string {
	if attrs == nil {
		return ""
	}
	return attrs[string(attrName)]
}
