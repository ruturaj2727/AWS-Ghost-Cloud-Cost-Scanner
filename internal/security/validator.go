package security

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/NotHarshhaa/aws-ghost/pkg/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Validator handles security validations
type Validator struct {
	config     types.SecurityConfig
	sts        *sts.Client
	apiTracker *APITracker
}

// NewValidator creates a new security validator
func NewValidator(cfg types.SecurityConfig, awsCfg aws.Config) *Validator {
	return &Validator{
		config:     cfg,
		sts:        sts.NewFromConfig(awsCfg),
		apiTracker: NewAPITracker(),
	}
}

// ValidateCredentials validates AWS credentials against security requirements
func (v *Validator) ValidateCredentials(ctx context.Context) (*types.CredentialInfo, error) {
	info := &types.CredentialInfo{
		Region:   "unknown",
		LastUsed: time.Now(),
	}

	// Get caller identity
	identity, err := v.sts.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to get caller identity: %w", err)
	}

	if identity.Account != nil {
		info.AccountID = *identity.Account
	}
	if identity.UserId != nil {
		info.UserID = *identity.UserId
		info.RootAccess = strings.HasPrefix(*identity.UserId, "AROA") // Root user ARN starts with AROA
	}
	if identity.Arn != nil {
		info.ARN = *identity.Arn
	}

	// Check MFA status
	if v.config.RequireMFA {
		mfaEnabled, err := v.checkMFAStatus(ctx, info.UserID)
		if err != nil {
			log.Printf("Warning: Could not verify MFA status: %v", err)
		} else {
			info.MFAEnabled = mfaEnabled
		}
	}

	// Validate against security requirements
	if err := v.validateCredentialRequirements(info); err != nil {
		return info, err
	}

	return info, nil
}

// ValidateRegion validates if the region is allowed based on security config
func (v *Validator) ValidateRegion(region string) error {
	if len(v.config.AllowedRegions) > 0 {
		for _, allowed := range v.config.AllowedRegions {
			if region == allowed {
				return nil
			}
		}
		return fmt.Errorf("region %s is not in allowed regions: %v", region, v.config.AllowedRegions)
	}

	if len(v.config.BlockedRegions) > 0 {
		for _, blocked := range v.config.BlockedRegions {
			if region == blocked {
				return fmt.Errorf("region %s is blocked", region)
			}
		}
	}

	return nil
}

// ValidateResourceAccess validates if scanning a resource type is allowed
func (v *Validator) ValidateResourceAccess(resourceType string) error {
	// In strict mode, only allow certain resource types
	if v.config.Level == types.SecurityLevelStrict {
		allowedTypes := map[string]bool{
			"ebs":       true,
			"eip":       true,
			"snapshots": true,
		}
		if !allowedTypes[resourceType] {
			return fmt.Errorf("resource type %s is not allowed in strict security mode", resourceType)
		}
	}
	return nil
}

// ValidateIdleDays checks if the idle days exceed security limits
func (v *Validator) ValidateIdleDays(idleDays int) error {
	if idleDays > v.config.MaxIdleDays {
		return fmt.Errorf("idle days %d exceeds maximum allowed %d", idleDays, v.config.MaxIdleDays)
	}
	return nil
}

// LogSecurityEvent logs a security event
func (v *Validator) LogSecurityEvent(event types.SecurityEvent) {
	if !v.config.AuditLogging {
		return
	}

	event.Timestamp = time.Now()
	event.Level = string(v.config.Level)

	log.Printf("[SECURITY] %s: %s - %s (Allowed: %t)",
		event.EventType, event.Message, event.Reason, event.Allowed)
}

// checkMFAStatus checks if MFA is enabled for the user
func (v *Validator) checkMFAStatus(ctx context.Context, userID string) (bool, error) {
	// Without IAM service access, we cannot reliably check MFA status
	// For now, we'll assume MFA is not verified unless explicitly configured
	// In a production environment, you would use IAM service to check MFA devices
	return false, nil
}

// validateCredentialRequirements validates credentials against security config
func (v *Validator) validateCredentialRequirements(info *types.CredentialInfo) error {
	if v.config.RequireMFA && !info.MFAEnabled {
		return fmt.Errorf("MFA is required but not enabled")
	}

	if !v.config.AllowRootAccess && info.RootAccess {
		return fmt.Errorf("root access is not allowed")
	}

	return nil
}

// ValidateScanConfig validates the overall scan configuration
func (v *Validator) ValidateScanConfig(scanConfig types.ScanConfig) error {
	// Validate region
	if err := v.ValidateRegion(scanConfig.Region); err != nil {
		return err
	}

	// Validate idle days
	if scanConfig.IdleDays > v.config.MaxIdleDays {
		return fmt.Errorf("scan idle days %d exceeds security maximum %d",
			scanConfig.IdleDays, v.config.MaxIdleDays)
	}

	return nil
}

// GetAPITracker returns the API tracker
func (v *Validator) GetAPITracker() *APITracker {
	return v.apiTracker
}
