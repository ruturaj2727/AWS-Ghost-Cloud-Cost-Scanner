package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/output"
	"github.com/NotHarshhaa/aws-ghost/internal/scanner"
	"github.com/NotHarshhaa/aws-ghost/internal/security"
	"github.com/NotHarshhaa/aws-ghost/internal/ui"
	"github.com/NotHarshhaa/aws-ghost/internal/webhook"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/spf13/cobra"
)

var (
	region        string
	allRegions    bool
	profile       string
	only          string
	skip          string
	outputFmt     string
	minCost       float64
	idleDays      int
	noColor       bool
	quiet         bool
	securityLevel string
	validatePerms bool
	auditLog      bool
	skipProtected bool
	tagFilter     string
	groupBy       string
	slackWebhook  string
	teamsWebhook  string
	notify        bool
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan AWS account for ghost resources",
	Long:  `Scan your AWS account for forgotten, idle, and wasteful resources.`,
	RunE:  runScan,
}

func init() {
	scanCmd.Flags().StringVarP(&region, "region", "r", "us-east-1", "AWS region to scan")
	scanCmd.Flags().BoolVar(&allRegions, "all-regions", false, "Scan all enabled regions")
	scanCmd.Flags().StringVarP(&profile, "profile", "p", "", "AWS named profile")
	scanCmd.Flags().StringVar(&only, "only", "", "Comma-separated resource types to scan")
	scanCmd.Flags().StringVar(&skip, "skip", "", "Comma-separated resource types to skip")
	scanCmd.Flags().StringVarP(&outputFmt, "output", "o", "text", "Output format: text, json, markdown, csv")
	scanCmd.Flags().Float64Var(&minCost, "min-cost", 0, "Only show resources above this monthly cost ($)")
	scanCmd.Flags().IntVar(&idleDays, "idle-days", 7, "Days of inactivity to consider a resource idle")
	scanCmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored terminal output")
	scanCmd.Flags().BoolVar(&quiet, "quiet", false, "Only print the summary line")

	// Security flags
	scanCmd.Flags().StringVar(&securityLevel, "security-level", "medium", "Security level: low, medium, high, strict")
	scanCmd.Flags().BoolVar(&validatePerms, "validate-permissions", true, "Validate AWS permissions before scanning")
	scanCmd.Flags().BoolVar(&auditLog, "audit-log", true, "Enable security audit logging")

	// Tag-based filtering flags
	scanCmd.Flags().BoolVar(&skipProtected, "skip-protected", false, "Skip resources with protection tags (keep=true, env=prod)")
	scanCmd.Flags().StringVar(&tagFilter, "tag-filter", "", "Only scan resources with specific tag (e.g. env=dev)")
	scanCmd.Flags().StringVar(&groupBy, "group-by", "", "Group results by tag (e.g. owner, project)")

	// Webhook notification flags
	scanCmd.Flags().StringVar(&slackWebhook, "slack-webhook", "", "Slack webhook URL for notifications")
	scanCmd.Flags().StringVar(&teamsWebhook, "teams-webhook", "", "Microsoft Teams webhook URL for notifications")
	scanCmd.Flags().BoolVar(&notify, "notify", false, "Send scan results to configured webhooks")
}

func runScan(cmd *cobra.Command, args []string) error {
	startTime := time.Now()

	// Parse include/exclude lists
	onlyList := parseList(only)
	skipList := parseList(skip)

	// Parse security level
	secLevel := types.SecurityLevel(securityLevel)
	secConfig := types.GetSecurityConfig(secLevel)
	secConfig.AuditLogging = auditLog
	secConfig.ValidatePermissions = validatePerms

	// Create AWS client
	client, err := aws.NewClient(profile, region)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	// Initialize security validator
	validator := security.NewValidator(secConfig, client.Config)

	// Validate credentials against security requirements
	credInfo, err := validator.ValidateCredentials(cmd.Context())
	if err != nil {
		validator.LogSecurityEvent(types.SecurityEvent{
			EventType: "credential_validation_failed",
			Message:   "Security validation failed",
			Reason:    err.Error(),
			Allowed:   false,
			User:      profile,
		})
		return fmt.Errorf("security validation failed: %w", err)
	}

	// Add credential source info
	credInfo.CredentialSource = client.CredentialSource

	// Display security information to build trust
	if !quiet {
		displaySecurityInfo(credInfo, secConfig, region)
	}

	// Get API tracker for transparency
	apiTracker := validator.GetAPITracker()

	// Log the initial STS call
	apiTracker.LogCall("STS", "GetCallerIdentity", region, true, "")

	// Log successful credential validation
	validator.LogSecurityEvent(types.SecurityEvent{
		EventType: "credential_validation_success",
		Message:   "Security validation passed",
		Allowed:   true,
		User:      profile,
		AccountID: credInfo.AccountID,
	})

	// Create scanner registry
	registry := scanner.NewRegistry(client)

	// Get filtered scanners
	scanners := registry.GetFiltered(onlyList, skipList)

	// Initialize progress indicator
	var resourceTypes []string
	for name := range scanners {
		resourceTypes = append(resourceTypes, name)
	}

	progress := ui.NewSimpleProgress(resourceTypes)
	if !quiet {
		progress.Start()
	}

	// Scan
	var allResources []types.Resource
	var scannedTypes []string

	for name, scn := range scanners {
		// Validate resource access against security policy
		if err := validator.ValidateResourceAccess(name); err != nil {
			validator.LogSecurityEvent(types.SecurityEvent{
				EventType: "resource_access_denied",
				Message:   fmt.Sprintf("Access to resource type %s denied by security policy", name),
				Reason:    err.Error(),
				Allowed:   false,
				User:      profile,
				AccountID: credInfo.AccountID,
				Region:    region,
			})
			fmt.Fprintf(cmd.ErrOrStderr(), "Security: %s\n", err)
			continue
		}

		config := types.ScanConfig{
			Region:   region,
			Profile:  profile,
			IdleDays: idleDays,
			MinCost:  minCost,
		}

		// Validate scan configuration
		if err := validator.ValidateScanConfig(config); err != nil {
			validator.LogSecurityEvent(types.SecurityEvent{
				EventType: "scan_config_invalid",
				Message:   fmt.Sprintf("Invalid scan configuration for %s", name),
				Reason:    err.Error(),
				Allowed:   false,
				User:      profile,
			})
			fmt.Fprintf(cmd.ErrOrStderr(), "Security: %s\n", err)
			continue
		}

		resources, err := scn.Scan(config)
		if err != nil {
			validator.LogSecurityEvent(types.SecurityEvent{
				EventType: "scan_failed",
				Message:   fmt.Sprintf("Failed to scan %s", name),
				Reason:    err.Error(),
				Allowed:   false,
				User:      profile,
				AccountID: credInfo.AccountID,
				Region:    region,
			})
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to scan %s: %v\n", name, err)
			continue
		}

		// Filter by min cost
		var filtered []types.Resource
		for _, r := range resources {
			if r.MonthlyCost >= minCost {
				filtered = append(filtered, r)
			}
		}

		allResources = append(allResources, filtered...)
		scannedTypes = append(scannedTypes, name)

		// Update progress
		if !quiet {
			progress.Update(name)
		}
	}

	// Complete progress
	if !quiet {
		progress.Complete()
	}

	// Apply tag-based filtering
	if skipProtected || tagFilter != "" {
		var tagFiltered []types.Resource
		for _, r := range allResources {
			if skipProtected {
				if v, ok := r.Metadata["keep"]; ok && v == "true" {
					continue
				}
				if v, ok := r.Metadata["env"]; ok && (v == "prod" || v == "production") {
					continue
				}
			}
			if tagFilter != "" {
				parts := strings.SplitN(tagFilter, "=", 2)
				if len(parts) == 2 {
					if v, ok := r.Metadata[parts[0]]; !ok || v != parts[1] {
						continue
					}
				}
			}
			tagFiltered = append(tagFiltered, r)
		}
		allResources = tagFiltered
	}

	// Calculate total cost
	totalCost := 0.0
	for _, r := range allResources {
		totalCost += r.MonthlyCost
	}

	// Create result
	result := types.ScanResult{
		AccountID:    client.AccountID,
		Region:       region,
		Resources:    allResources,
		TotalCost:    totalCost,
		ScanDuration: time.Since(startTime),
		ScannedTypes: scannedTypes,
	}

	// Use new UI formatter for text output, fall back to original for other formats
	if outputFmt == "text" {
		formatter := ui.NewFormatter(!noColor, quiet)
		outputStr, err := formatter.Format(result)
		if err != nil {
			return fmt.Errorf("failed to format output: %w", err)
		}
		fmt.Println(outputStr)
	} else {
		// Use original formatter for json, markdown, csv
		formatter, err := output.GetFormatter(outputFmt, noColor, quiet)
		if err != nil {
			return fmt.Errorf("failed to get formatter: %w", err)
		}

		outputStr, err := formatter.Format(result)
		if err != nil {
			return fmt.Errorf("failed to format output: %w", err)
		}

		// Write output
		if err := output.WriteOutput(outputStr, ""); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}

	// Display API call summary for transparency
	if !quiet && outputFmt == "text" {
		displayAPISummary(apiTracker)
	}

	// Send webhook notifications if requested
	if notify && (slackWebhook != "" || teamsWebhook != "") {
		notifier := webhook.NewWebhookNotifier(slackWebhook, teamsWebhook, "")
		results := []types.ScanResult{result}
		if slackWebhook != "" {
			if err := notifier.SendSlackNotification(results); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to send Slack notification: %v\n", err)
			} else if !quiet {
				fmt.Println("📢 Slack notification sent")
			}
		}
		if teamsWebhook != "" {
			if err := notifier.SendTeamsNotification(results); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to send Teams notification: %v\n", err)
			} else if !quiet {
				fmt.Println("📢 Teams notification sent")
			}
		}
	}

	return nil
}

func parseList(input string) []string {
	if input == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func displaySecurityInfo(credInfo *types.CredentialInfo, secConfig types.SecurityConfig, region string) {
	fmt.Printf("\n🔒 Security Verification\n")
	fmt.Printf("======================\n")
	fmt.Printf("Account ID: %s\n", credInfo.AccountID)
	fmt.Printf("User ARN: %s\n", credInfo.ARN)
	fmt.Printf("Credential Source: %s\n", credInfo.CredentialSource)
	fmt.Printf("Root Access: %s\n", map[bool]string{true: "⚠️  YES (not recommended)", false: "✅ No"}[credInfo.RootAccess])
	fmt.Printf("MFA Enabled: %s\n", map[bool]string{true: "✅ Yes", false: "ℹ️  Not verified"}[credInfo.MFAEnabled])
	fmt.Printf("Security Level: %s\n", string(secConfig.Level))
	fmt.Printf("Region: %s\n", region)
	fmt.Printf("\n🛡️  Safety Guarantees:\n")
	fmt.Printf("  • Credentials used locally only (never sent to external servers)\n")
	fmt.Printf("  • Read-only AWS API calls (Describe/List operations only)\n")
	fmt.Printf("  • No resource modifications or deletions\n")
	fmt.Printf("  • All API calls logged for transparency\n")
	fmt.Printf("\n")
}

func displayAPISummary(tracker *security.APITracker) {
	fmt.Printf("\n📊 API Call Summary\n")
	fmt.Printf("===================\n")
	fmt.Printf("%s", tracker.GetSummary())

	isReadOnly, writeOps := tracker.VerifyReadOnly()
	if isReadOnly {
		fmt.Printf("✅ All operations verified as read-only\n")
	} else {
		fmt.Printf("⚠️  Warning: Write operations detected:\n")
		for _, op := range writeOps {
			fmt.Printf("  - %s\n", op)
		}
	}
	fmt.Printf("\n")
}
