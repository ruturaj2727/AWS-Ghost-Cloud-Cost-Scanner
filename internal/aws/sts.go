package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// STSClient wraps the STS service
type STSClient struct {
	client *sts.Client
}

// NewSTSClient creates a new STS client
func NewSTSClient(cfg aws.Config) *STSClient {
	return &STSClient{
		client: sts.NewFromConfig(cfg),
	}
}

// GetAccountID retrieves the current AWS account ID
func (s *STSClient) GetAccountID() (string, error) {
	resp, err := s.client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	return *resp.Account, nil
}
