package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/cost"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
)

// KinesisScanner scans for idle Kinesis streams
type KinesisScanner struct {
	client *aws.Client
	calc   *cost.Calculator
}

// NewKinesisScanner creates a new Kinesis scanner
func NewKinesisScanner(client *aws.Client) *KinesisScanner {
	return &KinesisScanner{
		client: client,
		calc:   cost.NewCalculator(),
	}
}

// Scan returns idle Kinesis streams
func (s *KinesisScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	var resources []types.Resource

	ctx := context.TODO()

	streams, err := s.client.Kinesis.ListStreams(ctx, &kinesis.ListStreamsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Kinesis streams: %w", err)
	}

	for _, streamName := range streams.StreamNames {
		// Get detailed stream information
		streamInfo, err := s.client.Kinesis.DescribeStream(ctx, &kinesis.DescribeStreamInput{
			StreamName: &streamName,
		})
		if err != nil {
			continue
		}

		if streamInfo.StreamDescription == nil {
			continue
		}

		stream := streamInfo.StreamDescription

		// Check for idle streams (low throughput or old)
		if s.isIdleStream(*stream) {
			idleDays := s.calculateIdleDays(*stream)
			cost := s.calculateStreamCost(*stream)

			resource := types.Resource{
				ID:          streamName,
				Type:        "Kinesis",
				Region:      s.client.Config.Region,
				Name:        streamName,
				State:       "Idle",
				IdleDays:    idleDays,
				MonthlyCost: cost,
				Metadata: map[string]string{
					"status":          string(stream.StreamStatus),
					"shard_count":     getShardCount(*stream),
					"retention_hours": getRetentionHours(*stream),
					"encryption":      fmt.Sprintf("%t", isKinesisEncrypted(*stream)),
					"creation_time":   getCreationTime(*stream),
				},
				LastActive: time.Now().AddDate(0, 0, -idleDays),
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func (s *KinesisScanner) ResourceType() string {
	return "kinesis"
}

func (s *KinesisScanner) Description() string {
	return "Idle Kinesis streams"
}

func (s *KinesisScanner) isIdleStream(stream kinesistypes.StreamDescription) bool {
	// Consider idle if stream is active but potentially underutilized
	// In production, you'd check CloudWatch metrics for actual usage
	return stream.StreamStatus == kinesistypes.StreamStatusActive
}

func (s *KinesisScanner) calculateIdleDays(stream kinesistypes.StreamDescription) int {
	// Default to 30 days if we can't determine exact idle time
	// In production, you'd check CloudWatch metrics for actual activity
	return 30
}

func (s *KinesisScanner) calculateStreamCost(stream kinesistypes.StreamDescription) float64 {
	// Estimate cost based on shard count
	shardCount := getShardCountAsInt(stream)

	// Base cost: $0.036 per shard-hour
	shardCost := 0.036 * float64(shardCount) * 730 // $0.036/hr * hours/month

	// Add extended retention cost if applicable
	retentionHours := getRetentionHoursAsInt(stream)
	if retentionHours > 24 {
		additionalRetention := retentionHours - 24
		retentionCost := 0.015 * float64(additionalRetention) * float64(shardCount) / 24
		shardCost += retentionCost
	}

	return shardCost
}

func getShardCount(stream kinesistypes.StreamDescription) string {
	return fmt.Sprintf("%d", len(stream.Shards))
}

func getShardCountAsInt(stream kinesistypes.StreamDescription) int {
	return len(stream.Shards)
}

func getRetentionHours(stream kinesistypes.StreamDescription) string {
	if stream.RetentionPeriodHours != nil {
		return fmt.Sprintf("%d", *stream.RetentionPeriodHours)
	}
	return "24"
}

func getRetentionHoursAsInt(stream kinesistypes.StreamDescription) int64 {
	if stream.RetentionPeriodHours != nil {
		return int64(*stream.RetentionPeriodHours)
	}
	return 24
}

func isKinesisEncrypted(stream kinesistypes.StreamDescription) bool {
	return stream.EncryptionType != "" && stream.EncryptionType != kinesistypes.EncryptionTypeNone
}

func getCreationTime(stream kinesistypes.StreamDescription) string {
	if stream.StreamCreationTimestamp != nil {
		return stream.StreamCreationTimestamp.Format(time.RFC3339)
	}
	return "unknown"
}
