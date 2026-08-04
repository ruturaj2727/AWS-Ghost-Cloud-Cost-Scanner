package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/cost"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBScanner scans for idle DynamoDB tables
type DynamoDBScanner struct {
	client *aws.Client
	calc   *cost.Calculator
}

// NewDynamoDBScanner creates a new DynamoDB scanner
func NewDynamoDBScanner(client *aws.Client) *DynamoDBScanner {
	return &DynamoDBScanner{
		client: client,
		calc:   cost.NewCalculator(),
	}
}

// Scan returns idle DynamoDB tables
func (s *DynamoDBScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	var resources []types.Resource

	ctx := context.TODO()

	tables, err := s.client.DynamoDB.ListTables(ctx, &dynamodb.ListTablesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list DynamoDB tables: %w", err)
	}

	for _, tableName := range tables.TableNames {
		// Get detailed table information
		tableInfo, err := s.client.DynamoDB.DescribeTable(ctx, &dynamodb.DescribeTableInput{
			TableName: &tableName,
		})
		if err != nil {
			continue
		}

		if tableInfo.Table == nil {
			continue
		}

		table := tableInfo.Table

		// Check for idle tables (low read/write capacity or old)
		if s.isIdleTable(*table) {
			idleDays := s.calculateIdleDays(*table)
			cost := s.calculateTableCost(*table)

			resource := types.Resource{
				ID:          tableName,
				Type:        "DynamoDB",
				Region:      s.client.Config.Region,
				Name:        tableName,
				State:       "Idle",
				IdleDays:    idleDays,
				MonthlyCost: cost,
				Metadata: map[string]string{
					"table_mode":   getTableMode(*table),
					"status":       string(table.TableStatus),
					"item_count":   getItemCount(*table),
					"size_bytes":   getSizeBytes(*table),
					"billing_mode": getBillingMode(*table),
					"encryption":   fmt.Sprintf("%t", isDynamoDBEncrypted(*table)),
				},
				LastActive: time.Now().AddDate(0, 0, -idleDays),
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func (s *DynamoDBScanner) ResourceType() string {
	return "dynamodb"
}

func (s *DynamoDBScanner) Description() string {
	return "Idle DynamoDB tables"
}

func (s *DynamoDBScanner) isIdleTable(table dynamodbtypes.TableDescription) bool {
	// Consider idle if table is active but potentially underutilized
	// In production, you'd check CloudWatch metrics for actual usage
	return table.TableStatus == dynamodbtypes.TableStatusActive
}

func (s *DynamoDBScanner) calculateIdleDays(table dynamodbtypes.TableDescription) int {
	// Default to 30 days if we can't determine exact idle time
	// In production, you'd check CloudWatch metrics for actual activity
	return 30
}

func (s *DynamoDBScanner) calculateTableCost(table dynamodbtypes.TableDescription) float64 {
	// Estimate cost based on table mode and capacity
	if table.BillingModeSummary != nil && table.BillingModeSummary.BillingMode == dynamodbtypes.BillingModePayPerRequest {
		// On-demand pricing - estimate based on storage
		size := getSizeBytesAsInt(table)
		return 0.25 * float64(size) / (1024 * 1024 * 1024) // $0.25 per GB-month
	}

	// Provisioned mode - estimate based on capacity
	readCapacity := getReadCapacity(table)
	writeCapacity := getWriteCapacity(table)

	// Base cost estimation
	cost := 0.0
	if readCapacity > 0 {
		cost += 0.00065 * float64(readCapacity) * 730 // $0.00065 per RCU-hour
	}
	if writeCapacity > 0 {
		cost += 0.00013 * float64(writeCapacity) * 730 // $0.00013 per WCU-hour
	}

	// Add storage cost
	size := getSizeBytesAsInt(table)
	cost += 0.25 * float64(size) / (1024 * 1024 * 1024) // $0.25 per GB-month

	return cost
}

func getTableMode(table dynamodbtypes.TableDescription) string {
	if table.BillingModeSummary != nil {
		return string(table.BillingModeSummary.BillingMode)
	}
	return "provisioned"
}

func getItemCount(table dynamodbtypes.TableDescription) string {
	if table.ItemCount != nil {
		return fmt.Sprintf("%d", *table.ItemCount)
	}
	return "0"
}

func getSizeBytes(table dynamodbtypes.TableDescription) string {
	if table.TableSizeBytes != nil {
		return fmt.Sprintf("%d", *table.TableSizeBytes)
	}
	return "0"
}

func getSizeBytesAsInt(table dynamodbtypes.TableDescription) int64 {
	if table.TableSizeBytes != nil {
		return *table.TableSizeBytes
	}
	return 0
}

func getBillingMode(table dynamodbtypes.TableDescription) string {
	if table.BillingModeSummary != nil {
		return string(table.BillingModeSummary.BillingMode)
	}
	return "provisioned"
}

func isDynamoDBEncrypted(table dynamodbtypes.TableDescription) bool {
	if table.SSEDescription != nil {
		return true
	}
	return false
}

func getReadCapacity(table dynamodbtypes.TableDescription) int64 {
	if table.ProvisionedThroughput != nil && table.ProvisionedThroughput.ReadCapacityUnits != nil {
		return *table.ProvisionedThroughput.ReadCapacityUnits
	}
	return 0
}

func getWriteCapacity(table dynamodbtypes.TableDescription) int64 {
	if table.ProvisionedThroughput != nil && table.ProvisionedThroughput.WriteCapacityUnits != nil {
		return *table.ProvisionedThroughput.WriteCapacityUnits
	}
	return 0
}
