package cost

import (
	"fmt"
	"sync"
	"time"
)

// Calculator handles cost estimations for AWS resources
type Calculator struct {
	cache      map[string]float64
	cacheMutex sync.RWMutex
}

// NewCalculator creates a new cost calculator
func NewCalculator() *Calculator {
	return &Calculator{
		cache: make(map[string]float64),
	}
}

// getCached retrieves a cached cost calculation
func (c *Calculator) getCached(key string) (float64, bool) {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()
	val, ok := c.cache[key]
	return val, ok
}

// setCached stores a cost calculation in cache
func (c *Calculator) setCached(key string, value float64) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	c.cache[key] = value
}

// EBSVolumeCost calculates monthly cost for an EBS volume
// Prices in USD per GB/month (updated 2024)
func (c *Calculator) EBSVolumeCost(sizeGB int, volumeType string) float64 {
	cacheKey := fmt.Sprintf("ebs:%s:%d", volumeType, sizeGB)
	if cached, ok := c.getCached(cacheKey); ok {
		return cached
	}

	prices := map[string]float64{
		"gp2":      0.10,
		"gp3":      0.08, // $0.08/GB-month + baseline $0.00022/GB-month provisioned IOPS
		"io1":      0.125,
		"io2":      0.125,
		"st1":      0.045,
		"sc1":      0.025,
		"standard": 0.05,
	}

	price, ok := prices[volumeType]
	if !ok {
		price = 0.08 // default to gp3 (newer standard)
	}

	cost := float64(sizeGB) * price
	c.setCached(cacheKey, cost)
	return cost
}

// ElasticIPCost calculates monthly cost for an unattached Elastic IP
// $0.005 per hour when not attached
func (c *Calculator) ElasticIPCost() float64 {
	return 0.005 * 24 * 30 // $0.005/hr * 24hr * 30 days = $3.60/month
}

// LoadBalancerCost calculates monthly cost for a load balancer
// ALB/NLB: ~$0.0225/hr + LCU costs (updated 2024)
// CLB: ~$0.025/hr
func (c *Calculator) LoadBalancerCost(lbType string) float64 {
	cacheKey := fmt.Sprintf("lb:%s", lbType)
	if cached, ok := c.getCached(cacheKey); ok {
		return cached
	}

	prices := map[string]float64{
		"application": 16.20, // $0.0225/hr * 30 days + LCU costs
		"network":     16.20, // $0.0225/hr * 30 days + LCU costs
		"classic":     18.00, // $0.025/hr * 30 days
		"gateway":     18.00, // GWLB: $0.025/hr * 30 days
	}

	price, ok := prices[lbType]
	if !ok {
		price = 16.20 // default to ALB
	}

	c.setCached(cacheKey, price)
	return price
}

// NATGatewayCost calculates monthly cost for a NAT Gateway
// $0.045/hr + data processing costs
func (c *Calculator) NATGatewayCost() float64 {
	return 0.045 * 24 * 30 // $0.045/hr * 24hr * 30 days = $32.40/month
}

// SnapshotCost calculates monthly cost for a snapshot
// ~$0.05 per GB/month
func (c *Calculator) SnapshotCost(sizeGB int) float64 {
	if sizeGB < 0 {
		return 0
	}
	cacheKey := fmt.Sprintf("snapshot:%d", sizeGB)
	if cached, ok := c.getCached(cacheKey); ok {
		return cached
	}
	cost := float64(sizeGB) * 0.05
	c.setCached(cacheKey, cost)
	return cost
}

// ECRImageCost calculates monthly cost for ECR images
// ~$0.10 per GB/month
func (c *Calculator) ECRImageCost(sizeGB int) float64 {
	if sizeGB < 0 {
		return 0
	}
	cacheKey := fmt.Sprintf("ecr:%d", sizeGB)
	if cached, ok := c.getCached(cacheKey); ok {
		return cached
	}
	cost := float64(sizeGB) * 0.10
	c.setCached(cacheKey, cost)
	return cost
}

// EC2InstanceCost estimates monthly cost for an EC2 instance
// This is a rough estimate based on instance type
func (c *Calculator) EC2InstanceCost(instanceType string) float64 {
	cacheKey := fmt.Sprintf("ec2:%s", instanceType)
	if cached, ok := c.getCached(cacheKey); ok {
		return cached
	}

	prices := map[string]float64{
		"t2.micro":   8.47,
		"t2.small":   16.94,
		"t2.medium":  33.88,
		"t3.micro":   7.59,
		"t3.small":   15.18,
		"t3.medium":  30.36,
		"m5.large":   69.12,
		"m5.xlarge":  138.24,
		"m5.2xlarge": 276.48,
		"c5.large":   81.00,
		"c5.xlarge":  162.00,
		"c5.2xlarge": 324.00,
		"r5.large":   89.64,
		"r5.xlarge":  179.28,
		"r5.2xlarge": 358.56,
	}

	price, ok := prices[instanceType]
	if !ok {
		price = 30.00 // conservative default
	}

	c.setCached(cacheKey, price)
	return price
}

// CalculateIdleDays calculates the number of days a resource has been idle
func (c *Calculator) CalculateIdleDays(lastActive time.Time) int {
	if lastActive.IsZero() {
		return 999 // treat as very old
	}
	return int(time.Since(lastActive).Hours() / 24)
}
