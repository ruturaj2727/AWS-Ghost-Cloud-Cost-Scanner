package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Scanner scans for wasteful S3 resources
type S3Scanner struct {
	client *s3.Client
}

// NewS3Scanner creates a new S3 scanner
func NewS3Scanner(client interface{}) types.Scanner {
	return &S3Scanner{
		client: client.(*s3.Client),
	}
}

// ResourceType returns the type identifier for this scanner
func (s *S3Scanner) ResourceType() string {
	return "S3 Bucket"
}

// Description returns a human-readable description
func (s *S3Scanner) Description() string {
	return "Scans for wasteful S3 resources including empty buckets and old objects"
}

// Scan returns wasteful S3 resources
func (s *S3Scanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	ctx := context.Background()
	var resources []types.Resource

	// List all buckets
	buckets, err := s.listBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list S3 buckets: %w", err)
	}

	for _, bucket := range buckets {
		// Get bucket location to check if it's in the current region
		bucketRegion, err := s.getBucketLocation(ctx, *bucket.Name)
		if err != nil {
			continue // Skip buckets we can't get location for
		}

		if bucketRegion != config.Region {
			continue // Skip buckets not in current region
		}

		// Check for empty buckets
		isEmpty, lastModified, err := s.checkBucketEmpty(ctx, *bucket.Name)
		if err != nil {
			continue
		}

		if isEmpty {
			cost := s.calculateEmptyBucketCost(*bucket.Name)
			resources = append(resources, types.Resource{
				ID:          *bucket.Name,
				Type:        "S3 Bucket",
				Region:      config.Region,
				State:       "Empty",
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": fmt.Sprintf("Empty bucket (no objects)")},
				LastActive:  lastModified,
			})
			continue
		}

		// Check for old objects in buckets without lifecycle policies
		oldObjects, err := s.checkOldObjects(ctx, *bucket.Name, config.IdleDays)
		if err != nil {
			continue
		}

		if len(oldObjects) > 0 {
			cost := s.calculateOldObjectsCost(*bucket.Name, oldObjects)
			resources = append(resources, types.Resource{
				ID:          *bucket.Name,
				Type:        "S3 Bucket",
				Region:      config.Region,
				State:       "Contains old objects",
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": fmt.Sprintf("Contains %d objects older than %d days without lifecycle policy", len(oldObjects), config.IdleDays)},
				LastActive:  lastModified,
			})
		}

		// Check for incomplete multipart uploads
		incompleteUploads, err := s.checkIncompleteUploads(ctx, *bucket.Name)
		if err != nil {
			continue
		}

		if len(incompleteUploads) > 0 {
			cost := s.calculateMultipartUploadCost(*bucket.Name, incompleteUploads)
			resources = append(resources, types.Resource{
				ID:          *bucket.Name,
				Type:        "S3 Bucket",
				Region:      config.Region,
				State:       "Incomplete uploads",
				MonthlyCost: cost,
				Metadata:    map[string]string{"reason": fmt.Sprintf("Has %d incomplete multipart uploads", len(incompleteUploads))},
				LastActive:  lastModified,
			})
		}
	}

	return resources, nil
}

// listBuckets lists all S3 buckets
func (s *S3Scanner) listBuckets(ctx context.Context) ([]s3types.Bucket, error) {
	result, err := s.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}
	return result.Buckets, nil
}

// getBucketLocation gets the bucket region
func (s *S3Scanner) getBucketLocation(ctx context.Context, bucketName string) (string, error) {
	result, err := s.client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return "", err
	}

	// AWS returns empty string for us-east-1
	loc := result.LocationConstraint
	if loc == "" {
		return "us-east-1", nil
	}
	return string(loc), nil
}

// checkBucketEmpty checks if a bucket is empty
func (s *S3Scanner) checkBucketEmpty(ctx context.Context, bucketName string) (bool, time.Time, error) {
	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucketName),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return false, time.Time{}, err
	}

	if len(result.Contents) == 0 {
		// Get bucket creation date
		buckets, err := s.listBuckets(ctx)
		if err != nil {
			return true, time.Time{}, err
		}

		for _, bucket := range buckets {
			if *bucket.Name == bucketName {
				return true, *bucket.CreationDate, nil
			}
		}
	}

	return false, time.Time{}, nil
}

// checkOldObjects checks for objects older than specified days without lifecycle policy
func (s *S3Scanner) checkOldObjects(ctx context.Context, bucketName string, idleDays int) ([]s3types.Object, error) {
	// Check if bucket has lifecycle configuration
	_, err := s.client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucketName),
	})
	if err == nil {
		// Bucket has lifecycle policy, skip
		return nil, nil
	}

	var oldObjects []s3types.Object
	cutoffDate := time.Now().AddDate(0, 0, -idleDays)

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, obj := range page.Contents {
			if obj.LastModified.Before(cutoffDate) {
				oldObjects = append(oldObjects, obj)
			}
		}
	}

	return oldObjects, nil
}

// checkIncompleteUploads checks for incomplete multipart uploads
func (s *S3Scanner) checkIncompleteUploads(ctx context.Context, bucketName string) ([]s3types.MultipartUpload, error) {
	result, err := s.client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return nil, err
	}

	return result.Uploads, nil
}

// calculateEmptyBucketCost calculates cost for empty bucket
func (s *S3Scanner) calculateEmptyBucketCost(bucketName string) float64 {
	// Empty buckets still incur storage costs for bucket metadata
	// Approximately $0.01 per month per bucket
	return 0.01
}

// calculateOldObjectsCost calculates cost for old objects
func (s *S3Scanner) calculateOldObjectsCost(bucketName string, objects []s3types.Object) float64 {
	var totalSize int64
	for _, obj := range objects {
		totalSize += *obj.Size
	}

	// S3 Standard storage cost: ~$0.023 per GB per month
	gb := float64(totalSize) / (1024 * 1024 * 1024)
	return gb * 0.023
}

// calculateMultipartUploadCost calculates cost for incomplete uploads
func (s *S3Scanner) calculateMultipartUploadCost(bucketName string, uploads []s3types.MultipartUpload) float64 {
	// Incomplete uploads don't incur storage costs, but they indicate wasted operations
	// Small administrative cost for cleanup
	return float64(len(uploads)) * 0.01
}
