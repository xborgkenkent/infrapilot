package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Finding represents a security issue.
type Finding struct {
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Resource    string `json:"resource"`
	Description string `json:"description"`
}

// SecurityReport aggregates security findings.
type SecurityReport struct {
	Region   string    `json:"region"`
	Findings []Finding `json:"findings"`
}

// ScanSecurity runs read-only security checks.
func (c *Client) ScanSecurity(ctx context.Context) (*SecurityReport, error) {
	report := &SecurityReport{Region: c.Region}

	publicBuckets, err := c.findPublicS3Buckets(ctx)
	if err != nil {
		return nil, err
	}
	report.Findings = append(report.Findings, publicBuckets...)

	openSGs, err := c.findOpenSecurityGroups(ctx)
	if err != nil {
		return nil, err
	}
	report.Findings = append(report.Findings, openSGs...)

	noMFA, err := c.findIAMUsersWithoutMFA(ctx)
	if err != nil {
		return nil, err
	}
	report.Findings = append(report.Findings, noMFA...)

	publicEKS, err := c.findPublicEKSEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	report.Findings = append(report.Findings, publicEKS...)

	unencryptedRDS, err := c.findUnencryptedRDS(ctx)
	if err != nil {
		return nil, err
	}
	report.Findings = append(report.Findings, unencryptedRDS...)

	return report, nil
}

func (c *Client) findPublicS3Buckets(ctx context.Context) ([]Finding, error) {
	buckets, err := c.S3.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("list buckets: %w", err)
	}

	var findings []Finding
	for _, b := range buckets.Buckets {
		name := aws.ToString(b.Name)
		pab, err := c.S3.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: aws.String(name)})
		if err != nil {
			// No public access block configured is itself a risk.
			findings = append(findings, Finding{
				Severity:    "high",
				Category:    "s3_public_access",
				Resource:    name,
				Description: "S3 bucket has no public access block configuration",
			})
			continue
		}
		cfg := pab.PublicAccessBlockConfiguration
		if cfg != nil && !aws.ToBool(cfg.BlockPublicAcls) && !aws.ToBool(cfg.BlockPublicPolicy) {
			findings = append(findings, Finding{
				Severity:    "critical",
				Category:    "s3_public_access",
				Resource:    name,
				Description: "S3 bucket may allow public access",
			})
		}
	}
	return findings, nil
}

func (c *Client) findOpenSecurityGroups(ctx context.Context) ([]Finding, error) {
	out, err := c.EC2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return nil, fmt.Errorf("describe security groups: %w", err)
	}

	sensitivePorts := map[int32]string{
		22:   "SSH",
		3389: "RDP",
		3306: "MySQL",
		5432: "PostgreSQL",
		6379: "Redis",
	}

	var findings []Finding
	for _, sg := range out.SecurityGroups {
		for _, perm := range sg.IpPermissions {
			if perm.FromPort == nil {
				continue
			}
			port := *perm.FromPort
			label, sensitive := sensitivePorts[port]
			if !sensitive {
				continue
			}
			for _, ip := range perm.IpRanges {
				if aws.ToString(ip.CidrIp) == "0.0.0.0/0" {
					findings = append(findings, Finding{
						Severity:    "critical",
						Category:    "security_group",
						Resource:    aws.ToString(sg.GroupId),
						Description: fmt.Sprintf("Security group %s exposes %s (%d) to the internet", aws.ToString(sg.GroupName), label, port),
					})
				}
			}
		}
	}
	return findings, nil
}

func (c *Client) findIAMUsersWithoutMFA(ctx context.Context) ([]Finding, error) {
	users, err := c.IAM.ListUsers(ctx, &iam.ListUsersInput{})
	if err != nil {
		return nil, fmt.Errorf("list IAM users: %w", err)
	}

	var findings []Finding
	for _, user := range users.Users {
		mfa, err := c.IAM.ListMFADevices(ctx, &iam.ListMFADevicesInput{UserName: user.UserName})
		if err != nil {
			continue
		}
		if len(mfa.MFADevices) == 0 {
			findings = append(findings, Finding{
				Severity:    "high",
				Category:    "iam_mfa",
				Resource:    aws.ToString(user.UserName),
				Description: "IAM user does not have MFA enabled",
			})
		}
	}
	return findings, nil
}

func (c *Client) findPublicEKSEndpoints(ctx context.Context) ([]Finding, error) {
	clusters, err := c.EKS.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("list EKS clusters: %w", err)
	}

	var findings []Finding
	for _, name := range clusters.Clusters {
		desc, err := c.EKS.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(name)})
		if err != nil {
			continue
		}
		if desc.Cluster.ResourcesVpcConfig.EndpointPublicAccess {
			findings = append(findings, Finding{
				Severity:    "high",
				Category:    "eks_endpoint",
				Resource:    name,
				Description: "EKS API endpoint is publicly accessible",
			})
		}
	}
	return findings, nil
}

func (c *Client) findUnencryptedRDS(ctx context.Context) ([]Finding, error) {
	return c.scanRDSEncryption(ctx, false)
}

func (c *Client) scanRDSEncryption(ctx context.Context, encrypted bool) ([]Finding, error) {
	dbs, err := c.RDS.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		return nil, err
	}

	var findings []Finding
	for _, db := range dbs.DBInstances {
		if aws.ToBool(db.StorageEncrypted) == encrypted {
			continue
		}
		findings = append(findings, Finding{
			Severity:    "medium",
			Category:    "rds_encryption",
			Resource:    aws.ToString(db.DBInstanceIdentifier),
			Description: "RDS instance storage is not encrypted",
		})
	}
	return findings, nil
}

// CheckPublicAccess returns resources with public network exposure.
func (c *Client) CheckPublicAccess(ctx context.Context) (*SecurityReport, error) {
	report := &SecurityReport{Region: c.Region}

	s3Findings, err := c.findPublicS3Buckets(ctx)
	if err != nil {
		return nil, err
	}
	report.Findings = append(report.Findings, s3Findings...)

	sgFindings, err := c.findOpenSecurityGroups(ctx)
	if err != nil {
		return nil, err
	}
	report.Findings = append(report.Findings, sgFindings...)

	// Public RDS instances
	dbs, err := c.RDS.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		return nil, err
	}
	for _, db := range dbs.DBInstances {
		if aws.ToBool(db.PubliclyAccessible) {
			report.Findings = append(report.Findings, Finding{
				Severity:    "critical",
				Category:    "rds_public",
				Resource:    aws.ToString(db.DBInstanceIdentifier),
				Description: "RDS instance is publicly accessible",
			})
		}
	}

	return report, nil
}

// IAMReview summarizes IAM posture.
type IAMReview struct {
	UsersWithoutMFA []string `json:"users_without_mfa"`
	AdminLikeUsers  []string `json:"admin_like_users"`
	TotalUsers      int      `json:"total_users"`
}

// ReviewIAM performs IAM posture review.
func (c *Client) ReviewIAM(ctx context.Context) (*IAMReview, error) {
	users, err := c.IAM.ListUsers(ctx, &iam.ListUsersInput{})
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	review := &IAMReview{TotalUsers: len(users.Users)}
	for _, user := range users.Users {
		name := aws.ToString(user.UserName)
		mfa, err := c.IAM.ListMFADevices(ctx, &iam.ListMFADevicesInput{UserName: user.UserName})
		if err == nil && len(mfa.MFADevices) == 0 {
			review.UsersWithoutMFA = append(review.UsersWithoutMFA, name)
		}

		attached, err := c.IAM.ListAttachedUserPolicies(ctx, &iam.ListAttachedUserPoliciesInput{UserName: user.UserName})
		if err != nil {
			continue
		}
		for _, p := range attached.AttachedPolicies {
			if strings.Contains(strings.ToLower(aws.ToString(p.PolicyName)), "admin") {
				review.AdminLikeUsers = append(review.AdminLikeUsers, name)
			}
		}
	}
	return review, nil
}

func (r *SecurityReport) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *IAMReview) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
