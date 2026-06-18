package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Client wraps AWS service clients for a single region.
type Client struct {
	Region string
	EC2    *ec2.Client
	EKS    *eks.Client
	RDS    *rds.Client
	S3     *s3.Client
	ELBv2  *elasticloadbalancingv2.Client
	IAM    *iam.Client
	STS    *sts.Client
}

// NewClient loads AWS configuration from the default credential chain.
func NewClient(ctx context.Context, region string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	return &Client{
		Region: region,
		EC2:    ec2.NewFromConfig(cfg),
		EKS:    eks.NewFromConfig(cfg),
		RDS:    rds.NewFromConfig(cfg),
		S3:     s3.NewFromConfig(cfg),
		ELBv2:  elasticloadbalancingv2.NewFromConfig(cfg),
		IAM:    iam.NewFromConfig(cfg),
		STS:    sts.NewFromConfig(cfg),
	}, nil
}

// AccountID returns the current AWS account ID.
func (c *Client) AccountID(ctx context.Context) (string, error) {
	out, err := c.STS.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	if out.Account == nil {
		return "", fmt.Errorf("empty account ID")
	}
	return *out.Account, nil
}
