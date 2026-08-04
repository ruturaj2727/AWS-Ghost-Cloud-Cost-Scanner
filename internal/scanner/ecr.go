package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/cost"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// ECRScanner scans for unused ECR images
type ECRScanner struct {
	client *aws.Client
	calc   *cost.Calculator
}

// NewECRScanner creates a new ECR scanner
func NewECRScanner(client *aws.Client) *ECRScanner {
	return &ECRScanner{
		client: client,
		calc:   cost.NewCalculator(),
	}
}

// Scan returns unused ECR images (untagged or not pulled in 90+ days)
func (s *ECRScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
	var resources []types.Resource

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// List all repositories
	repoResp, err := s.client.ECR.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe ECR repositories: %w\nTip: Ensure you have ecr:DescribeRepositories permission", err)
	}

	for _, repo := range repoResp.Repositories {
		if repo.RepositoryName == nil {
			continue
		}

		// List images in the repository
		imgResp, err := s.client.ECR.DescribeImages(ctx, &ecr.DescribeImagesInput{
			RepositoryName: repo.RepositoryName,
			Filter:         &ecrtypes.DescribeImagesFilter{TagStatus: ecrtypes.TagStatusAny},
		})
		if err != nil {
			continue
		}

		for _, img := range imgResp.ImageDetails {
			if img.ImageDigest == nil {
				continue
			}

			isGhost := false
			reason := ""

			// Check if untagged
			if len(img.ImageTags) == 0 {
				isGhost = true
				reason = "untagged image"
			}

			// Check if not pulled in 90+ days
			if !isGhost && img.LastRecordedPullTime != nil {
				daysSincePull := int(time.Since(*img.LastRecordedPullTime).Hours() / 24)
				if daysSincePull > 90 {
					isGhost = true
					reason = fmt.Sprintf("last pulled %d days ago", daysSincePull)
				}
			} else if !isGhost && img.LastRecordedPullTime == nil && img.ImagePushedAt != nil {
				// Never pulled, check push time
				daysSincePush := int(time.Since(*img.ImagePushedAt).Hours() / 24)
				if daysSincePush > 90 {
					isGhost = true
					reason = fmt.Sprintf("never pulled, pushed %d days ago", daysSincePush)
				}
			}

			if !isGhost {
				continue
			}

			sizeBytes := int64(0)
			if img.ImageSizeInBytes != nil {
				sizeBytes = *img.ImageSizeInBytes
			}
			sizeGB := int(sizeBytes / (1024 * 1024 * 1024))
			if sizeGB == 0 {
				sizeGB = 1 // minimum 1 GB for cost calc
			}

			idleDays := 0
			var lastActive time.Time
			if img.ImagePushedAt != nil {
				idleDays = int(time.Since(*img.ImagePushedAt).Hours() / 24)
				lastActive = *img.ImagePushedAt
			}

			tag := "untagged"
			if len(img.ImageTags) > 0 {
				tag = img.ImageTags[0]
			}

			resource := types.Resource{
				ID:          fmt.Sprintf("%s:%s", *repo.RepositoryName, tag),
				Type:        "ecr",
				Region:      s.client.Config.Region,
				Name:        *repo.RepositoryName,
				State:       reason,
				IdleDays:    idleDays,
				MonthlyCost: s.calc.ECRImageCost(sizeGB),
				Metadata: map[string]string{
					"repository": *repo.RepositoryName,
					"digest":     *img.ImageDigest,
					"size_mb":    fmt.Sprintf("%d", sizeBytes/(1024*1024)),
					"reason":     reason,
				},
				LastActive: lastActive,
			}

			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func (s *ECRScanner) ResourceType() string {
	return "ecr"
}

func (s *ECRScanner) Description() string {
	return "Unused ECR images"
}
