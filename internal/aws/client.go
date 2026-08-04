package aws

import (
	"context"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// Client wraps AWS SDK clients
type Client struct {
	Config           aws.Config
	EC2              *ec2.Client
	ELB              *elasticloadbalancing.Client
	ELBv2            *elasticloadbalancingv2.Client
	RDS              *rds.Client
	ECR              *ecr.Client
	Lambda           *lambda.Client
	CloudWatch       *cloudwatch.Client
	CloudWatchLogs   *cloudwatchlogs.Client
	S3               *s3.Client
	CloudFront       *cloudfront.Client
	AutoScaling      *autoscaling.Client
	ECS              *ecs.Client
	EKS              *eks.Client
	ElastiCache      *elasticache.Client
	OpenSearch       *opensearch.Client
	Redshift         *redshift.Client
	DynamoDB         *dynamodb.Client
	Kinesis          *kinesis.Client
	SQS              *sqs.Client
	SNS              *sns.Client
	AccountID        string
	CredentialSource string
}

// NewClient creates a new AWS client with the given profile and region
func NewClient(profile, region string) (*Client, error) {
	var cfg aws.Config
	var err error

	// Detect credential source
	credentialSource := detectCredentialSource(profile)

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		config.WithRetryer(func() aws.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) {
				o.MaxAttempts = 3
				o.MaxBackoff = 20 * time.Second
			})
		}),
	}

	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err = config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	// Get account ID
	stsSvc := NewSTSClient(cfg)
	accountID, err := stsSvc.GetAccountID()
	if err != nil {
		accountID = "unknown"
	}

	return &Client{
		Config:           cfg,
		EC2:              ec2.NewFromConfig(cfg),
		ELB:              elasticloadbalancing.NewFromConfig(cfg),
		ELBv2:            elasticloadbalancingv2.NewFromConfig(cfg),
		RDS:              rds.NewFromConfig(cfg),
		ECR:              ecr.NewFromConfig(cfg),
		Lambda:           lambda.NewFromConfig(cfg),
		CloudWatch:       cloudwatch.NewFromConfig(cfg),
		CloudWatchLogs:   cloudwatchlogs.NewFromConfig(cfg),
		S3:               s3.NewFromConfig(cfg),
		CloudFront:       cloudfront.NewFromConfig(cfg),
		AutoScaling:      autoscaling.NewFromConfig(cfg),
		ECS:              ecs.NewFromConfig(cfg),
		EKS:              eks.NewFromConfig(cfg),
		ElastiCache:      elasticache.NewFromConfig(cfg),
		OpenSearch:       opensearch.NewFromConfig(cfg),
		Redshift:         redshift.NewFromConfig(cfg),
		DynamoDB:         dynamodb.NewFromConfig(cfg),
		Kinesis:          kinesis.NewFromConfig(cfg),
		SQS:              sqs.NewFromConfig(cfg),
		SNS:              sns.NewFromConfig(cfg),
		AccountID:        accountID,
		CredentialSource: credentialSource,
	}, nil
}

// detectCredentialSource determines where AWS credentials are coming from
func detectCredentialSource(profile string) string {
	// Check environment variables
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" || os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		return "environment_variables"
	}
	if os.Getenv("AWS_SESSION_TOKEN") != "" {
		return "session_token"
	}
	if os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE") != "" {
		return "web_identity"
	}

	// If profile is specified, it's from AWS credentials file
	if profile != "" {
		return "aws_profile:" + profile
	}

	// Default to default profile or IAM role
	return "default_credential_chain"
}

// NewClientForRegion creates a new AWS client for a specific region
func NewClientForRegion(profile, region string) (*Client, error) {
	return NewClient(profile, region)
}
