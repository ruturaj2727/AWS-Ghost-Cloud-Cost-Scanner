package security

import (
	"context"
	"fmt"
	"time"

	"github.com/NotHarshhaa/aws-ghost/pkg/types"
)

// Middleware provides security middleware for AWS operations
type Middleware struct {
	validator *Validator
	config    types.SecurityConfig
}

// NewMiddleware creates a new security middleware
func NewMiddleware(validator *Validator) *Middleware {
	return &Middleware{
		validator: validator,
		config:    validator.config,
	}
}

// SecureScan wraps scanner operations with security checks
func (m *Middleware) SecureScan(ctx context.Context, scannerType string, scanFunc func(context.Context, types.ScanConfig) ([]types.Resource, error), config types.ScanConfig) ([]types.Resource, error) {
	// Log scan attempt
	m.validator.LogSecurityEvent(types.SecurityEvent{
		EventType: "scan_attempt",
		Message:   fmt.Sprintf("Attempting to scan %s", scannerType),
		Allowed:   true,
	})

	// Validate resource access
	if err := m.validator.ValidateResourceAccess(scannerType); err != nil {
		m.validator.LogSecurityEvent(types.SecurityEvent{
			EventType: "resource_access_denied",
			Message:   fmt.Sprintf("Access denied for %s", scannerType),
			Reason:    err.Error(),
			Allowed:   false,
		})
		return nil, fmt.Errorf("security policy denied access to %s: %w", scannerType, err)
	}

	// Validate scan configuration
	if err := m.validator.ValidateScanConfig(config); err != nil {
		m.validator.LogSecurityEvent(types.SecurityEvent{
			EventType: "scan_config_invalid",
			Message:   fmt.Sprintf("Invalid scan config for %s", scannerType),
			Reason:    err.Error(),
			Allowed:   false,
		})
		return nil, fmt.Errorf("invalid scan configuration: %w", err)
	}

	// Execute scan with timeout if in strict mode
	if m.config.Level == types.SecurityLevelStrict {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
	}

	// Perform the scan
	resources, err := scanFunc(ctx, config)
	if err != nil {
		m.validator.LogSecurityEvent(types.SecurityEvent{
			EventType: "scan_failed",
			Message:   fmt.Sprintf("Scan failed for %s", scannerType),
			Reason:    err.Error(),
			Allowed:   false,
		})
		return nil, err
	}

	// Filter resources based on security policy
	filtered := m.filterResources(resources)

	// Log successful scan
	m.validator.LogSecurityEvent(types.SecurityEvent{
		EventType: "scan_success",
		Message:   fmt.Sprintf("Successfully scanned %s, found %d resources", scannerType, len(filtered)),
		Allowed:   true,
	})

	return filtered, nil
}

// filterResources filters resources based on security policy
func (m *Middleware) filterResources(resources []types.Resource) []types.Resource {
	var filtered []types.Resource

	for _, resource := range resources {
		// Check if resource exceeds maximum idle days
		if err := m.validator.ValidateIdleDays(resource.IdleDays); err != nil {
			m.validator.LogSecurityEvent(types.SecurityEvent{
				EventType: "resource_filtered",
				Message:   fmt.Sprintf("Resource %s filtered: %s", resource.ID, err.Error()),
				ResourceID: resource.ID,
				Allowed:   false,
				Reason:    err.Error(),
			})
			continue
		}

		// In strict mode, apply additional filters
		if m.config.Level == types.SecurityLevelStrict {
			if !m.isResourceAllowedInStrictMode(resource) {
				m.validator.LogSecurityEvent(types.SecurityEvent{
					EventType: "resource_strict_mode_filtered",
					Message:   fmt.Sprintf("Resource %s filtered by strict mode policy", resource.ID),
					ResourceID: resource.ID,
					Allowed:   false,
					Reason:    "strict mode policy",
				})
				continue
			}
		}

		filtered = append(filtered, resource)
	}

	return filtered
}

// isResourceAllowedInStrictMode checks if a resource is allowed in strict security mode
func (m *Middleware) isResourceAllowedInStrictMode(resource types.Resource) bool {
	// In strict mode, only allow resources with certain characteristics
	switch resource.Type {
	case "ebs":
		// Only allow unencrypted volumes if explicitly configured
		if encrypted, ok := resource.Metadata["encrypted"]; ok && encrypted == "false" {
			return false
		}
	case "eip":
		// Only allow EIPs in approved regions
		for _, region := range m.config.AllowedRegions {
			if resource.Region == region {
				return true
			}
		}
		return false
	}

	return true
}

// GetSecurityMetadata returns security metadata for the scan
func (m *Middleware) GetSecurityMetadata() map[string]interface{} {
	return map[string]interface{}{
		"security_level":       string(m.config.Level),
		"require_mfa":          m.config.RequireMFA,
		"allow_root_access":    m.config.AllowRootAccess,
		"max_idle_days":        m.config.MaxIdleDays,
		"audit_logging":        m.config.AuditLogging,
		"validate_permissions": m.config.ValidatePermissions,
		"encrypt_output":       m.config.EncryptOutput,
		"allowed_regions":      m.config.AllowedRegions,
		"blocked_regions":      m.config.BlockedRegions,
		"scan_timestamp":       time.Now().Format(time.RFC3339),
	}
}
