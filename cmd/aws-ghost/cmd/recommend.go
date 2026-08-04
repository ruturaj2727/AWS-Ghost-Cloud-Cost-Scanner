package cmd

import (
	"fmt"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/scanner"
	"github.com/NotHarshhaa/aws-ghost/internal/security"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/spf13/cobra"
)

var (
	recommendOutput string
	detailedRecs    bool
)

var recommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: "Get optimization recommendations for AWS resources",
	Long:  `Analyze ghost resources and provide actionable recommendations for cost optimization.`,
	RunE:  runRecommend,
}

func init() {
	recommendCmd.Flags().StringVarP(&recommendOutput, "output", "o", "text", "Output format: text, json")
	recommendCmd.Flags().BoolVar(&detailedRecs, "detailed", false, "Show detailed recommendations with reasoning")
}

func runRecommend(cmd *cobra.Command, args []string) error {
	client, err := aws.NewClient(profile, region)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	secConfig := types.GetSecurityConfig(types.SecurityLevelMedium)
	validator := security.NewValidator(secConfig, client.Config)

	credInfo, err := validator.ValidateCredentials(cmd.Context())
	if err != nil {
		return fmt.Errorf("security validation failed: %w", err)
	}

	registry := scanner.NewRegistry(client)
	scanners := registry.GetFiltered(nil, nil)

	var allResources []types.Resource
	for name, scn := range scanners {
		config := types.ScanConfig{
			Region:   region,
			Profile:  profile,
			IdleDays: idleDays,
			MinCost:  0,
		}

		resources, err := scn.Scan(config)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to scan %s: %v\n", name, err)
			continue
		}

		allResources = append(allResources, resources...)
	}

	// Generate recommendations
	recommendations := generateRecommendations(allResources)

	fmt.Printf("\n💡 Optimization Recommendations\n")
	fmt.Printf("================================\n")
	fmt.Printf("Account: %s\n", credInfo.AccountID)
	fmt.Printf("Region: %s\n", region)
	fmt.Printf("\n")

	// Group recommendations by priority
	highPriority := []string{}
	mediumPriority := []string{}
	lowPriority := []string{}

	for _, rec := range recommendations {
		switch rec.Priority {
		case "high":
			highPriority = append(highPriority, rec.Message)
		case "medium":
			mediumPriority = append(mediumPriority, rec.Message)
		case "low":
			lowPriority = append(lowPriority, rec.Message)
		}
	}

	if len(highPriority) > 0 {
		fmt.Printf("🔴 High Priority (Immediate Action)\n")
		fmt.Printf("─────────────────────────────────\n")
		for _, rec := range highPriority {
			fmt.Printf("  • %s\n", rec)
		}
		fmt.Printf("\n")
	}

	if len(mediumPriority) > 0 {
		fmt.Printf("🟡 Medium Priority (Plan for Next Sprint)\n")
		fmt.Printf("────────────────────────────────────────\n")
		for _, rec := range mediumPriority {
			fmt.Printf("  • %s\n", rec)
		}
		fmt.Printf("\n")
	}

	if len(lowPriority) > 0 {
		fmt.Printf("🟢 Low Priority (Technical Debt)\n")
		fmt.Printf("──────────────────────────────────\n")
		for _, rec := range lowPriority {
			fmt.Printf("  • %s\n", rec)
		}
		fmt.Printf("\n")
	}

	// Show summary
	totalSavings := calculatePotentialSavings(allResources)
	fmt.Printf("💰 Estimated Potential Savings: $%.2f/month\n", totalSavings)
	fmt.Printf("📊 Total Resources Analyzed: %d\n", len(allResources))
	fmt.Printf("\n")

	return nil
}

type Recommendation struct {
	Message  string
	Priority string
	Resource string
}

func generateRecommendations(resources []types.Resource) []Recommendation {
	var recommendations []Recommendation
	resourceCounts := make(map[string]int)
	costByType := make(map[string]float64)

	for _, r := range resources {
		resourceCounts[r.Type]++
		costByType[r.Type] += r.MonthlyCost
	}

	// EBS Volume recommendations
	if count, ok := resourceCounts["ebs"]; ok && count > 0 {
		cost := costByType["ebs"]
		if cost > 50 {
			recommendations = append(recommendations, Recommendation{
				Message:  fmt.Sprintf("Delete %d unattached EBS volumes to save $%.2f/month. Consider enabling automatic volume deletion on instance termination.", count, cost),
				Priority: "high",
				Resource: "ebs",
			})
		}
	}

	// Elastic IP recommendations
	if count, ok := resourceCounts["eip"]; ok && count > 0 {
		cost := costByType["eip"]
		recommendations = append(recommendations, Recommendation{
			Message:  fmt.Sprintf("Release %d unused Elastic IPs to save $%.2f/month. Use VPC IP addressing instead.", count, cost),
			Priority: "high",
			Resource: "eip",
		})
	}

	// Load Balancer recommendations
	if count, ok := resourceCounts["loadbalancer"]; ok && count > 0 {
		cost := costByType["loadbalancer"]
		if cost > 30 {
			recommendations = append(recommendations, Recommendation{
				Message:  fmt.Sprintf("Remove %d idle load balancers to save $%.2f/month. Consider using Application Load Balancer only when needed.", count, cost),
				Priority: "medium",
				Resource: "loadbalancer",
			})
		}
	}

	// NAT Gateway recommendations
	if count, ok := resourceCounts["nat"]; ok && count > 0 {
		cost := costByType["nat"]
		recommendations = append(recommendations, Recommendation{
			Message:  fmt.Sprintf("Delete %d unused NAT Gateways to save $%.2f/month. Each NAT Gateway costs ~$32/month even at zero traffic.", count, cost),
			Priority: "high",
			Resource: "nat",
		})
	}

	// Snapshot recommendations
	if count, ok := resourceCounts["snapshot"]; ok && count > 5 {
		cost := costByType["snapshot"]
		recommendations = append(recommendations, Recommendation{
			Message:  fmt.Sprintf("Clean up %d old snapshots to save $%.2f/month. Implement a snapshot retention policy (e.g., keep 7 daily, 4 weekly, 3 monthly).", count, cost),
			Priority: "medium",
			Resource: "snapshot",
		})
	}

	// ECR recommendations
	if count, ok := resourceCounts["ecr"]; ok && count > 0 {
		cost := costByType["ecr"]
		recommendations = append(recommendations, Recommendation{
			Message:  fmt.Sprintf("Remove %d untagged or old ECR images to save $%.2f/month. Set up lifecycle policies for automatic cleanup.", count, cost),
			Priority: "low",
			Resource: "ecr",
		})
	}

	// Lambda recommendations
	if count, ok := resourceCounts["lambda"]; ok && count > 0 {
		recommendations = append(recommendations, Recommendation{
			Message:  fmt.Sprintf("Review %d idle Lambda functions. Consider removing or enabling provisioned concurrency only for production functions.", count),
			Priority: "low",
			Resource: "lambda",
		})
	}

	// S3 recommendations
	if count, ok := resourceCounts["s3"]; ok && count > 0 {
		cost := costByType["s3"]
		recommendations = append(recommendations, Recommendation{
			Message:  fmt.Sprintf("Optimize %d S3 buckets to save $%.2f/month. Enable lifecycle policies for versioning and set appropriate storage classes.", count, cost),
			Priority: "medium",
			Resource: "s3",
		})
	}

	// CloudFront recommendations
	if count, ok := resourceCounts["cloudfront"]; ok && count > 0 {
		cost := costByType["cloudfront"]
		recommendations = append(recommendations, Recommendation{
			Message:  fmt.Sprintf("Disable %d unused CloudFront distributions to save $%.2f/month. Each distribution costs ~$0.50/month even without traffic.", count, cost),
			Priority: "medium",
			Resource: "cloudfront",
		})
	}

	// General recommendations
	if len(resources) > 20 {
		recommendations = append(recommendations, Recommendation{
			Message:  "Implement Infrastructure as Code (Terraform/CloudFormation) to track resource ownership and lifecycle.",
			Priority: "low",
			Resource: "general",
		})
	}

	recommendations = append(recommendations, Recommendation{
		Message:  "Set up regular automated scans using `aws-ghost budget --check` to catch waste early.",
		Priority: "medium",
		Resource: "general",
	})

	return recommendations
}

func calculatePotentialSavings(resources []types.Resource) float64 {
	total := 0.0
	for _, r := range resources {
		total += r.MonthlyCost
	}
	return total
}
