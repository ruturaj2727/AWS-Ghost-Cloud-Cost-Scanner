package types

import "time"

// SecurityLevel represents the security strictness level
type SecurityLevel string

const (
	SecurityLevelLow    SecurityLevel = "low"
	SecurityLevelMedium SecurityLevel = "medium"
	SecurityLevelHigh   SecurityLevel = "high"
	SecurityLevelStrict SecurityLevel = "strict"
)

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	Level               SecurityLevel `json:"level"`
	RequireMFA          bool          `json:"require_mfa"`
	AllowRootAccess     bool          `json:"allow_root_access"`
	MaxIdleDays         int           `json:"max_idle_days"`
	AuditLogging        bool          `json:"audit_logging"`
	ValidatePermissions bool          `json:"validate_permissions"`
	EncryptOutput       bool          `json:"encrypt_output"`
	AllowedRegions      []string      `json:"allowed_regions,omitempty"`
	BlockedRegions      []string      `json:"blocked_regions,omitempty"`
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		Level:               SecurityLevelMedium,
		RequireMFA:          false,
		AllowRootAccess:     false,
		MaxIdleDays:         90,
		AuditLogging:        true,
		ValidatePermissions: true,
		EncryptOutput:       false,
		AllowedRegions:      []string{},
		BlockedRegions:      []string{},
	}
}

// GetSecurityConfig returns security config for the given level
func GetSecurityConfig(level SecurityLevel) SecurityConfig {
	switch level {
	case SecurityLevelLow:
		return SecurityConfig{
			Level:               SecurityLevelLow,
			RequireMFA:          false,
			AllowRootAccess:     true,
			MaxIdleDays:         365,
			AuditLogging:        false,
			ValidatePermissions: false,
			EncryptOutput:       false,
		}
	case SecurityLevelMedium:
		return SecurityConfig{
			Level:               SecurityLevelMedium,
			RequireMFA:          false,
			AllowRootAccess:     false,
			MaxIdleDays:         90,
			AuditLogging:        true,
			ValidatePermissions: true,
			EncryptOutput:       false,
		}
	case SecurityLevelHigh:
		return SecurityConfig{
			Level:               SecurityLevelHigh,
			RequireMFA:          true,
			AllowRootAccess:     false,
			MaxIdleDays:         30,
			AuditLogging:        true,
			ValidatePermissions: true,
			EncryptOutput:       true,
		}
	case SecurityLevelStrict:
		return SecurityConfig{
			Level:               SecurityLevelStrict,
			RequireMFA:          true,
			AllowRootAccess:     false,
			MaxIdleDays:         7,
			AuditLogging:        true,
			ValidatePermissions: true,
			EncryptOutput:       true,
			AllowedRegions:      []string{"us-east-1", "us-west-2", "eu-west-1"},
		}
	default:
		return DefaultSecurityConfig()
	}
}

// SecurityEvent represents a security-related event
type SecurityEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	EventType  string    `json:"event_type"`
	Level      string    `json:"level"`
	Message    string    `json:"message"`
	AccountID  string    `json:"account_id,omitempty"`
	Region     string    `json:"region,omitempty"`
	ResourceID string    `json:"resource_id,omitempty"`
	User       string    `json:"user,omitempty"`
	Action     string    `json:"action,omitempty"`
	Allowed    bool      `json:"allowed"`
	Reason     string    `json:"reason,omitempty"`
}

// CredentialInfo holds information about AWS credentials
type CredentialInfo struct {
	HasAccessKey     bool      `json:"has_access_key"`
	HasSecretKey     bool      `json:"has_secret_key"`
	HasSessionToken  bool      `json:"has_session_token"`
	ProfileName      string    `json:"profile_name,omitempty"`
	Region           string    `json:"region"`
	AccountID        string    `json:"account_id,omitempty"`
	UserID           string    `json:"user_id,omitempty"`
	ARN              string    `json:"arn,omitempty"`
	MFAEnabled       bool      `json:"mfa_enabled"`
	RootAccess       bool      `json:"root_access"`
	LastUsed         time.Time `json:"last_used"`
	CredentialSource string    `json:"credential_source,omitempty"`
}
