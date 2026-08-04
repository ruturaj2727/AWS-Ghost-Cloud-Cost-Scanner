package types

import "time"

// Resource represents a scanned AWS resource
type Resource struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Region       string            `json:"region"`
	Name         string            `json:"name,omitempty"`
	State        string            `json:"state,omitempty"`
	IdleDays     int               `json:"idle_days"`
	MonthlyCost  float64           `json:"monthly_cost"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	LastActive   time.Time         `json:"last_active,omitempty"`
}

// ScanResult contains the results of a scan
type ScanResult struct {
	AccountID    string            `json:"account_id"`
	Region       string            `json:"region"`
	Resources    []Resource        `json:"resources"`
	TotalCost    float64           `json:"total_cost"`
	ScanDuration time.Duration     `json:"scan_duration"`
	ScannedTypes []string          `json:"scanned_types"`
}

// ScanConfig holds configuration for a scan
type ScanConfig struct {
	Region     string
	Profile    string
	AllRegions bool
	Only       []string
	Skip       []string
	MinCost    float64
	IdleDays   int
}
