package security

import (
	"fmt"
	"sync"
	"time"
)

// APICall represents a single AWS API call made during the scan
type APICall struct {
	Service    string    `json:"service"`
	Operation  string    `json:"operation"`
	Timestamp  time.Time `json:"timestamp"`
	Region     string    `json:"region"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
}

// APITracker tracks all AWS API calls for transparency
type APITracker struct {
	calls []APICall
	mu    sync.Mutex
}

// NewAPITracker creates a new API tracker
func NewAPITracker() *APITracker {
	return &APITracker{
		calls: make([]APICall, 0),
	}
}

// LogCall logs an API call
func (t *APITracker) LogCall(service, operation, region string, success bool, errMsg string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.calls = append(t.calls, APICall{
		Service:   service,
		Operation: operation,
		Timestamp: time.Now(),
		Region:    region,
		Success:   success,
		Error:     errMsg,
	})
}

// GetCalls returns all logged API calls
func (t *APITracker) GetCalls() []APICall {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make([]APICall, len(t.calls))
	copy(result, t.calls)
	return result
}

// GetSummary returns a summary of API calls
func (t *APITracker) GetSummary() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.calls) == 0 {
		return "No API calls made"
	}

	total := len(t.calls)
	successful := 0
	failed := 0
	services := make(map[string]int)
	operations := make(map[string]int)

	for _, call := range t.calls {
		if call.Success {
			successful++
		} else {
			failed++
		}
		services[call.Service]++
		operations[call.Operation]++
	}

	summary := fmt.Sprintf("Total API Calls: %d (Successful: %d, Failed: %d)\n", total, successful, failed)
	summary += "Services Used:\n"
	for svc, count := range services {
		summary += fmt.Sprintf("  - %s: %d calls\n", svc, count)
	}
	summary += "Operations:\n"
	for op, count := range operations {
		summary += fmt.Sprintf("  - %s: %d calls\n", op, count)
	}

	return summary
}

// VerifyReadOnly verifies that all operations are read-only
func (t *APITracker) VerifyReadOnly() (bool, []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var writeOps []string
	writeOperations := map[string]bool{
		"Create":  true,
		"Delete":  true,
		"Update":  true,
		"Put":     true,
		"Post":    true,
		"Modify":  true,
		"Attach":  true,
		"Detach":  true,
		"Start":   true,
		"Stop":    true,
		"Reboot":  true,
		"Terminate": true,
		"Run":     true,
		"Execute": true,
	}

	for _, call := range t.calls {
		for writeOp := range writeOperations {
			if contains(call.Operation, writeOp) {
				writeOps = append(writeOps, fmt.Sprintf("%s.%s", call.Service, call.Operation))
			}
		}
	}

	return len(writeOps) == 0, writeOps
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr)))
}
