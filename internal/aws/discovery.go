package aws

import (
	"context"
	"encoding/json"
	"fmt"
)

// ResourceSummary describes discovered infrastructure counts.
type ResourceSummary struct {
	AccountID      string `json:"account_id"`
	Region         string `json:"region"`
	VPCs           int    `json:"vpcs"`
	EC2Instances   int    `json:"ec2_instances"`
	EKSClusters    int    `json:"eks_clusters"`
	RDSInstances   int    `json:"rds_instances"`
	S3Buckets      int    `json:"s3_buckets"`
	LoadBalancers  int    `json:"load_balancers"`
	SecurityGroups int    `json:"security_groups"`
}

// ResourceDetail holds a single discovered resource.
type ResourceDetail struct {
	Type   string            `json:"type"`
	ID     string            `json:"id"`
	Name   string            `json:"name,omitempty"`
	Status string            `json:"status,omitempty"`
	Tags   map[string]string `json:"tags,omitempty"`
	Extra  map[string]string `json:"extra,omitempty"`
}

// Inventory is the full discovery result.
type Inventory struct {
	Summary  ResourceSummary  `json:"summary"`
	Resources []ResourceDetail `json:"resources"`
}

// Discover inventories AWS resources in the configured region.
func (c *Client) Discover(ctx context.Context) (*Inventory, error) {
	inv := &Inventory{
		Summary: ResourceSummary{Region: c.Region},
	}

	accountID, err := c.AccountID(ctx)
	if err != nil {
		return nil, fmt.Errorf("get account ID: %w", err)
	}
	inv.Summary.AccountID = accountID

	vpcs, err := c.listVPCs(ctx)
	if err != nil {
		return nil, err
	}
	inv.Summary.VPCs = len(vpcs)
	inv.Resources = append(inv.Resources, vpcs...)

	instances, err := c.listEC2(ctx)
	if err != nil {
		return nil, err
	}
	inv.Summary.EC2Instances = len(instances)
	inv.Resources = append(inv.Resources, instances...)

	clusters, err := c.listEKS(ctx)
	if err != nil {
		return nil, err
	}
	inv.Summary.EKSClusters = len(clusters)
	inv.Resources = append(inv.Resources, clusters...)

	databases, err := c.listRDS(ctx)
	if err != nil {
		return nil, err
	}
	inv.Summary.RDSInstances = len(databases)
	inv.Resources = append(inv.Resources, databases...)

	buckets, err := c.listS3(ctx)
	if err != nil {
		return nil, err
	}
	inv.Summary.S3Buckets = len(buckets)
	inv.Resources = append(inv.Resources, buckets...)

	lbs, err := c.listLoadBalancers(ctx)
	if err != nil {
		return nil, err
	}
	inv.Summary.LoadBalancers = len(lbs)
	inv.Resources = append(inv.Resources, lbs...)

	sgs, err := c.listSecurityGroups(ctx)
	if err != nil {
		return nil, err
	}
	inv.Summary.SecurityGroups = len(sgs)
	inv.Resources = append(inv.Resources, sgs...)

	return inv, nil
}

// ToJSON serializes the inventory.
func (inv *Inventory) ToJSON() (string, error) {
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ArchitectureExplanation generates a human-readable architecture summary.
func (inv *Inventory) ArchitectureExplanation() string {
	s := inv.Summary
	var flow string
	switch {
	case s.LoadBalancers > 0 && s.EKSClusters > 0 && s.RDSInstances > 0:
		flow = "Load Balancer → EKS → RDS"
	case s.LoadBalancers > 0 && s.EC2Instances > 0:
		flow = "Load Balancer → EC2"
	case s.EKSClusters > 0:
		flow = "EKS workloads"
	default:
		flow = "No dominant traffic pattern detected"
	}

	return fmt.Sprintf(`Infrastructure Overview (Account %s, Region %s):

- %d VPC(s)
- %d EKS cluster(s)
- %d RDS instance(s)
- %d EC2 instance(s)
- %d S3 bucket(s)
- %d load balancer(s)
- %d security group(s)

Likely Traffic Flow:
%s`, s.AccountID, s.Region, s.VPCs, s.EKSClusters, s.RDSInstances,
		s.EC2Instances, s.S3Buckets, s.LoadBalancers, s.SecurityGroups, flow)
}
