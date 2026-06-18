package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// WasteItem describes a potentially wasteful resource.
type WasteItem struct {
	Type        string  `json:"type"`
	ResourceID  string  `json:"resource_id"`
	Description string  `json:"description"`
	EstMonthly  float64 `json:"estimated_monthly_usd"`
}

// CostReport summarizes cost optimization opportunities.
type CostReport struct {
	Region              string      `json:"region"`
	WasteItems          []WasteItem `json:"waste_items"`
	EstimatedMonthlyUSD float64     `json:"estimated_monthly_savings_usd"`
}

// DetectWaste finds idle or unused resources.
func (c *Client) DetectWaste(ctx context.Context) (*CostReport, error) {
	report := &CostReport{Region: c.Region}

	idleEC2, err := c.findIdleEC2(ctx)
	if err != nil {
		return nil, err
	}
	report.WasteItems = append(report.WasteItems, idleEC2...)

	unattached, err := c.findUnattachedEBS(ctx)
	if err != nil {
		return nil, err
	}
	report.WasteItems = append(report.WasteItems, unattached...)

	unusedEIPs, err := c.findUnusedEIPs(ctx)
	if err != nil {
		return nil, err
	}
	report.WasteItems = append(report.WasteItems, unusedEIPs...)

	staleSnaps, err := c.findStaleSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	report.WasteItems = append(report.WasteItems, staleSnaps...)

	for _, item := range report.WasteItems {
		report.EstimatedMonthlyUSD += item.EstMonthly
	}
	return report, nil
}

func (c *Client) findIdleEC2(ctx context.Context) ([]WasteItem, error) {
	out, err := c.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []types.Filter{{
			Name:   aws.String("instance-state-name"),
			Values: []string{"stopped"},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("describe stopped instances: %w", err)
	}

	var items []WasteItem
	for _, res := range out.Reservations {
		for _, inst := range res.Instances {
			items = append(items, WasteItem{
				Type:        "ec2_stopped",
				ResourceID:  aws.ToString(inst.InstanceId),
				Description: "EC2 instance is stopped but may still incur EBS costs",
				EstMonthly:  10,
			})
		}
	}
	return items, nil
}

func (c *Client) findUnattachedEBS(ctx context.Context) ([]WasteItem, error) {
	out, err := c.EC2.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
		Filters: []types.Filter{{
			Name:   aws.String("status"),
			Values: []string{"available"},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("describe volumes: %w", err)
	}

	var items []WasteItem
	for _, vol := range out.Volumes {
		gb := float64(aws.ToInt32(vol.Size))
		items = append(items, WasteItem{
			Type:        "ebs_unattached",
			ResourceID:  aws.ToString(vol.VolumeId),
			Description: fmt.Sprintf("Unattached EBS volume (%d GB)", int(gb)),
			EstMonthly:  gb * 0.10,
		})
	}
	return items, nil
}

func (c *Client) findUnusedEIPs(ctx context.Context) ([]WasteItem, error) {
	out, err := c.EC2.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, fmt.Errorf("describe elastic IPs: %w", err)
	}

	var items []WasteItem
	for _, addr := range out.Addresses {
		if addr.AssociationId == nil {
			items = append(items, WasteItem{
				Type:        "eip_unused",
				ResourceID:  aws.ToString(addr.AllocationId),
				Description: "Unassociated Elastic IP",
				EstMonthly:  3.65,
			})
		}
	}
	return items, nil
}

func (c *Client) findStaleSnapshots(ctx context.Context) ([]WasteItem, error) {
	out, err := c.EC2.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{
		OwnerIds: []string{"self"},
	})
	if err != nil {
		return nil, fmt.Errorf("describe snapshots: %w", err)
	}

	cutoff := time.Now().AddDate(-1, 0, 0)
	var items []WasteItem
	for _, snap := range out.Snapshots {
		if snap.StartTime != nil && snap.StartTime.Before(cutoff) {
			gb := float64(aws.ToInt32(snap.VolumeSize))
			items = append(items, WasteItem{
				Type:        "snapshot_stale",
				ResourceID:  aws.ToString(snap.SnapshotId),
				Description: fmt.Sprintf("Snapshot older than 1 year (%d GB)", int(gb)),
				EstMonthly:  gb * 0.05,
			})
		}
	}
	return items, nil
}

// CostAnalysis returns a high-level cost analysis summary.
func (c *Client) CostAnalysis(ctx context.Context) (string, error) {
	report, err := c.DetectWaste(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`Cost Analysis (Region %s):

Potential waste items: %d
Estimated monthly savings: $%.2f

Top categories:
%s`, report.Region, len(report.WasteItems), report.EstimatedMonthlyUSD, summarizeWaste(report)), nil
}

func summarizeWaste(report *CostReport) string {
	counts := map[string]int{}
	for _, item := range report.WasteItems {
		counts[item.Type]++
	}
	if len(counts) == 0 {
		return "- No obvious waste detected"
	}
	var lines string
	for k, v := range counts {
		lines += fmt.Sprintf("- %s: %d\n", k, v)
	}
	return lines
}

func (r *CostReport) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *CostReport) SavingsReport() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Potential Savings Report (Region %s)\n\n", r.Region)
	if len(r.WasteItems) == 0 {
		b.WriteString("No waste detected.\n")
		return b.String()
	}
	for _, item := range r.WasteItems {
		fmt.Fprintf(&b, "- [%s] %s: %s (~$%.2f/mo)\n", item.Type, item.ResourceID, item.Description, item.EstMonthly)
	}
	fmt.Fprintf(&b, "\nEstimated Monthly Savings: $%.2f\n", r.EstimatedMonthlyUSD)
	return b.String()
}
