package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/NotHarshhaa/aws-ghost/internal/aws"
	"github.com/NotHarshhaa/aws-ghost/internal/security"
	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/spf13/cobra"
)

var securityCmd = &cobra.Command{
	Use:   "security",
	Short: "Manage security settings and view security information",
	Long:  `Manage security settings, view current security configuration, and audit security events.`,
}

var securityInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show current security configuration",
	Long:  `Display the current security configuration and available security levels.`,
	RunE:  runSecurityInfo,
}

var securityLevelsCmd = &cobra.Command{
	Use:   "levels",
	Short: "Show available security levels",
	Long:  `Display all available security levels and their configurations.`,
	RunE:  runSecurityLevels,
}

var securityAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Run a security audit of your AWS credentials",
	Long:  `Perform a comprehensive security audit of your AWS credentials and configuration.`,
	RunE:  runSecurityAudit,
}

func init() {
	rootCmd.AddCommand(securityCmd)
	securityCmd.AddCommand(securityInfoCmd)
	securityCmd.AddCommand(securityLevelsCmd)
	securityCmd.AddCommand(securityAuditCmd)

	// Add flags for audit command
	securityAuditCmd.Flags().StringVarP(&region, "region", "r", "us-east-1", "AWS region to use for audit")
	securityAuditCmd.Flags().StringVarP(&profile, "profile", "p", "", "AWS named profile")
}

func runSecurityInfo(cmd *cobra.Command, args []string) error {
	// Get current security level from flags or default
	level := types.SecurityLevel(securityLevel)
	if level == "" {
		level = types.SecurityLevelMedium
	}

	config := types.GetSecurityConfig(level)

	fmt.Printf("Current Security Configuration\n")
	fmt.Printf("=============================\n\n")
	fmt.Printf("Security Level: %s\n\n", string(level))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "SETTING\tVALUE\tDESCRIPTION\n")
	fmt.Fprintf(w, "--------\t-----\t-----------\n")
	fmt.Fprintf(w, "MFA Required\t%t\tRequire Multi-Factor Authentication\n", config.RequireMFA)
	fmt.Fprintf(w, "Root Access\t%t\tAllow root account access\n", config.AllowRootAccess)
	fmt.Fprintf(w, "Max Idle Days\t%d\tMaximum days before resource is considered ghost\n", config.MaxIdleDays)
	fmt.Fprintf(w, "Audit Logging\t%t\tEnable security audit logging\n", config.AuditLogging)
	fmt.Fprintf(w, "Permission Validation\t%t\tValidate AWS permissions before scanning\n", config.ValidatePermissions)
	fmt.Fprintf(w, "Output Encryption\t%t\tEncrypt scan output\n", config.EncryptOutput)

	if len(config.AllowedRegions) > 0 {
		fmt.Fprintf(w, "Allowed Regions\t%v\tRegions where scanning is allowed\n", config.AllowedRegions)
	}

	if len(config.BlockedRegions) > 0 {
		fmt.Fprintf(w, "Blocked Regions\t%v\tRegions where scanning is blocked\n", config.BlockedRegions)
	}

	w.Flush()

	fmt.Printf("\nSecurity Recommendations:\n")
	fmt.Printf("- Use 'high' or 'strict' levels for production environments\n")
	fmt.Printf("- Enable MFA for all AWS accounts\n")
	fmt.Printf("- Regularly review audit logs for security events\n")
	fmt.Printf("- Consider restricting regions to those you actually use\n")

	return nil
}

func runSecurityLevels(cmd *cobra.Command, args []string) error {
	levels := []types.SecurityLevel{
		types.SecurityLevelLow,
		types.SecurityLevelMedium,
		types.SecurityLevelHigh,
		types.SecurityLevelStrict,
	}

	fmt.Printf("Available Security Levels\n")
	fmt.Printf("========================\n\n")

	for _, level := range levels {
		config := types.GetSecurityConfig(level)

		fmt.Printf("Level: %s\n", string(level))
		fmt.Printf("Description: %s\n", getLevelDescription(level))
		fmt.Printf("Best for: %s\n\n", getLevelUseCase(level))

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  SETTING\tVALUE\n")
		fmt.Fprintf(w, "  --------\t-----\n")
		fmt.Fprintf(w, "  MFA Required\t%t\n", config.RequireMFA)
		fmt.Fprintf(w, "  Root Access\t%t\n", config.AllowRootAccess)
		fmt.Fprintf(w, "  Max Idle Days\t%d\n", config.MaxIdleDays)
		fmt.Fprintf(w, "  Audit Logging\t%t\n", config.AuditLogging)
		fmt.Fprintf(w, "  Permission Validation\t%t\n", config.ValidatePermissions)
		fmt.Fprintf(w, "  Output Encryption\t%t\n", config.EncryptOutput)
		w.Flush()

		if len(config.AllowedRegions) > 0 {
			fmt.Printf("  Allowed Regions\t%v\n", config.AllowedRegions)
		}

		fmt.Printf("\n%s\n\n", getLevelWarning(level))
	}

	fmt.Printf("Usage Examples:\n")
	fmt.Printf("  aws-ghost scan --security-level low\n")
	fmt.Printf("  aws-ghost scan --security-level high --validate-permissions\n")
	fmt.Printf("  aws-ghost scan --security-level strict --audit-log\n")

	return nil
}

func getLevelDescription(level types.SecurityLevel) string {
	switch level {
	case types.SecurityLevelLow:
		return "Minimal security restrictions for development/testing"
	case types.SecurityLevelMedium:
		return "Balanced security for general use"
	case types.SecurityLevelHigh:
		return "Enhanced security for production environments"
	case types.SecurityLevelStrict:
		return "Maximum security with comprehensive restrictions"
	default:
		return "Unknown level"
	}
}

func getLevelUseCase(level types.SecurityLevel) string {
	switch level {
	case types.SecurityLevelLow:
		return "Development, testing, personal projects"
	case types.SecurityLevelMedium:
		return "General scanning, non-production environments"
	case types.SecurityLevelHigh:
		return "Production environments, compliance requirements"
	case types.SecurityLevelStrict:
		return "Highly regulated environments, maximum security"
	default:
		return "Unknown use case"
	}
}

func getLevelWarning(level types.SecurityLevel) string {
	switch level {
	case types.SecurityLevelLow:
		return "⚠️  Low security level allows root access and has minimal restrictions"
	case types.SecurityLevelMedium:
		return "✅ Recommended for most use cases with good security balance"
	case types.SecurityLevelHigh:
		return "🔒 High security level requires MFA and has stricter controls"
	case types.SecurityLevelStrict:
		return "🛡️  Strict mode severely limits operations and regions"
	default:
		return ""
	}
}

func runSecurityAudit(cmd *cobra.Command, args []string) error {
	fmt.Printf("🔍 Security Audit\n")
	fmt.Printf("================\n\n")

	// Get region from flag or default
	auditRegion := region
	if auditRegion == "" {
		auditRegion = "us-east-1"
	}

	// Create AWS client
	client, err := aws.NewClient(profile, auditRegion)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	// Initialize security validator with medium level
	secConfig := types.GetSecurityConfig(types.SecurityLevelMedium)
	validator := security.NewValidator(secConfig, client.Config)

	// Validate credentials
	credInfo, err := validator.ValidateCredentials(cmd.Context())
	if err != nil {
		return fmt.Errorf("credential validation failed: %w", err)
	}

	// Add credential source
	credInfo.CredentialSource = client.CredentialSource

	// Display credential information
	fmt.Printf("📋 Credential Information\n")
	fmt.Printf("-------------------------\n")
	fmt.Printf("Account ID: %s\n", credInfo.AccountID)
	fmt.Printf("User ARN: %s\n", credInfo.ARN)
	fmt.Printf("Credential Source: %s\n", credInfo.CredentialSource)
	fmt.Printf("Root Access: %s\n", map[bool]string{true: "⚠️  YES", false: "✅ No"}[credInfo.RootAccess])
	fmt.Printf("MFA Enabled: %s\n", map[bool]string{true: "✅ Yes", false: "⚠️  Not verified"}[credInfo.MFAEnabled])
	fmt.Printf("\n")

	// Security recommendations
	fmt.Printf("💡 Security Recommendations\n")
	fmt.Printf("---------------------------\n")

	if credInfo.RootAccess {
		fmt.Printf("⚠️  CRITICAL: You are using root account access. This is not recommended.\n")
		fmt.Printf("   Recommendation: Create an IAM user with appropriate permissions instead.\n")
	} else {
		fmt.Printf("✅ Good: You are not using root account access.\n")
	}

	if !credInfo.MFAEnabled {
		fmt.Printf("⚠️  WARNING: MFA is not verified for your credentials.\n")
		fmt.Printf("   Recommendation: Enable MFA on your IAM user for enhanced security.\n")
	} else {
		fmt.Printf("✅ Good: MFA is enabled on your account.\n")
	}

	switch credInfo.CredentialSource {
	case "environment_variables":
		fmt.Printf("ℹ️  Note: Credentials from environment variables. Ensure these are not logged or committed.\n")
	case "default_credential_chain":
		fmt.Printf("✅ Good: Using default credential chain (IAM role or profile).\n")
	default:
		fmt.Printf("ℹ️  Note: Using AWS profile: %s\n", credInfo.CredentialSource)
	}

	fmt.Printf("\n")
	fmt.Printf("🛡️  Tool Safety Guarantees\n")
	fmt.Printf("--------------------------\n")
	fmt.Printf("✅ Credentials are used locally only (never sent to external servers)\n")
	fmt.Printf("✅ Only read-only AWS API calls (Describe/List operations)\n")
	fmt.Printf("✅ No resource modifications or deletions\n")
	fmt.Printf("✅ All API calls are logged for transparency\n")
	fmt.Printf("✅ Open-source code auditable by anyone\n")
	fmt.Printf("\n")
	fmt.Printf("📊 Required IAM Permissions\n")
	fmt.Printf("---------------------------\n")
	fmt.Printf("The tool requires only read-only permissions:\n")
	fmt.Printf("  • ec2:Describe*\n")
	fmt.Printf("  • elasticloadbalancing:Describe*\n")
	fmt.Printf("  • rds:Describe*\n")
	fmt.Printf("  • ecr:Describe* and ecr:ListImages\n")
	fmt.Printf("  • lambda:List* and lambda:GetFunction\n")
	fmt.Printf("  • logs:Describe*\n")
	fmt.Printf("  • cloudwatch:GetMetricStatistics and cloudwatch:ListMetrics\n")
	fmt.Printf("\n")
	fmt.Printf("✅ Audit complete. Your credentials are safe to use with aws-ghost.\n")

	return nil
}
