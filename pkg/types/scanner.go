package types

// Scanner defines the interface for resource scanners
type Scanner interface {
	// Scan returns a list of ghost resources for this type
	Scan(config ScanConfig) ([]Resource, error)
	
	// ResourceType returns the type identifier for this scanner
	ResourceType() string
	
	// Description returns a human-readable description
	Description() string
}
