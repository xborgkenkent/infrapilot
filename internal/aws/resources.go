package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func tagsToMap(tags []types.Tag) map[string]string {
	m := make(map[string]string, len(tags))
	for _, t := range tags {
		if t.Key != nil && t.Value != nil {
			m[*t.Key] = *t.Value
		}
	}
	return m
}

func nameFromTags(tags map[string]string) string {
	if n, ok := tags["Name"]; ok {
		return n
	}
	return ""
}

func (c *Client) listVPCs(ctx context.Context) ([]ResourceDetail, error) {
	out, err := c.EC2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return nil, fmt.Errorf("describe VPCs: %w", err)
	}

	var resources []ResourceDetail
	for _, vpc := range out.Vpcs {
		tags := tagsToMap(vpc.Tags)
		resources = append(resources, ResourceDetail{
			Type:   "vpc",
			ID:     aws.ToString(vpc.VpcId),
			Name:   nameFromTags(tags),
			Status: string(vpc.State),
			Tags:   tags,
			Extra: map[string]string{
				"cidr": aws.ToString(vpc.CidrBlock),
			},
		})
	}
	return resources, nil
}

func (c *Client) listEC2(ctx context.Context) ([]ResourceDetail, error) {
	out, err := c.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("describe EC2 instances: %w", err)
	}

	var resources []ResourceDetail
	for _, res := range out.Reservations {
		for _, inst := range res.Instances {
			tags := tagsToMap(inst.Tags)
			resources = append(resources, ResourceDetail{
				Type:   "ec2",
				ID:     aws.ToString(inst.InstanceId),
				Name:   nameFromTags(tags),
				Status: string(inst.State.Name),
				Tags:   tags,
				Extra: map[string]string{
					"type":   string(inst.InstanceType),
					"az":     aws.ToString(inst.Placement.AvailabilityZone),
					"vpc_id": aws.ToString(inst.VpcId),
				},
			})
		}
	}
	return resources, nil
}

func (c *Client) listEKS(ctx context.Context) ([]ResourceDetail, error) {
	out, err := c.EKS.ListClusters(ctx, &eks.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("list EKS clusters: %w", err)
	}

	var resources []ResourceDetail
	for _, name := range out.Clusters {
		desc, err := c.EKS.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(name)})
		if err != nil {
			return nil, fmt.Errorf("describe EKS cluster %s: %w", name, err)
		}
		cluster := desc.Cluster
		resources = append(resources, ResourceDetail{
			Type:   "eks",
			ID:     aws.ToString(cluster.Arn),
			Name:   aws.ToString(cluster.Name),
			Status: string(cluster.Status),
			Extra: map[string]string{
				"version":        aws.ToString(cluster.Version),
				"endpoint_public": fmt.Sprintf("%t", cluster.ResourcesVpcConfig.EndpointPublicAccess),
			},
		})
	}
	return resources, nil
}

func (c *Client) listRDS(ctx context.Context) ([]ResourceDetail, error) {
	out, err := c.RDS.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("describe RDS instances: %w", err)
	}

	var resources []ResourceDetail
	for _, db := range out.DBInstances {
		resources = append(resources, ResourceDetail{
			Type:   "rds",
			ID:     aws.ToString(db.DBInstanceIdentifier),
			Name:   aws.ToString(db.DBInstanceIdentifier),
			Status: aws.ToString(db.DBInstanceStatus),
			Extra: map[string]string{
				"engine":   aws.ToString(db.Engine),
				"encrypted": fmt.Sprintf("%t", aws.ToBool(db.StorageEncrypted)),
				"public":   fmt.Sprintf("%t", aws.ToBool(db.PubliclyAccessible)),
			},
		})
	}
	return resources, nil
}

func (c *Client) listS3(ctx context.Context) ([]ResourceDetail, error) {
	out, err := c.S3.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("list S3 buckets: %w", err)
	}

	var resources []ResourceDetail
	for _, b := range out.Buckets {
		resources = append(resources, ResourceDetail{
			Type: "s3",
			ID:   aws.ToString(b.Name),
			Name: aws.ToString(b.Name),
			Extra: map[string]string{
				"created": b.CreationDate.String(),
			},
		})
	}
	return resources, nil
}

func (c *Client) listLoadBalancers(ctx context.Context) ([]ResourceDetail, error) {
	out, err := c.ELBv2.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{})
	if err != nil {
		return nil, fmt.Errorf("describe load balancers: %w", err)
	}

	var resources []ResourceDetail
	for _, lb := range out.LoadBalancers {
		resources = append(resources, ResourceDetail{
			Type:   "load_balancer",
			ID:     aws.ToString(lb.LoadBalancerArn),
			Name:   aws.ToString(lb.LoadBalancerName),
			Status: string(lb.State.Code),
			Extra: map[string]string{
				"type": string(lb.Type),
				"dns":  aws.ToString(lb.DNSName),
			},
		})
	}
	return resources, nil
}

func (c *Client) listSecurityGroups(ctx context.Context) ([]ResourceDetail, error) {
	out, err := c.EC2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return nil, fmt.Errorf("describe security groups: %w", err)
	}

	var resources []ResourceDetail
	for _, sg := range out.SecurityGroups {
		tags := tagsToMap(sg.Tags)
		resources = append(resources, ResourceDetail{
			Type:   "security_group",
			ID:     aws.ToString(sg.GroupId),
			Name:   aws.ToString(sg.GroupName),
			Tags:   tags,
			Extra: map[string]string{
				"vpc_id": aws.ToString(sg.VpcId),
			},
		})
	}
	return resources, nil
}
