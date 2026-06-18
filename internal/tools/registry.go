package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kent/infrapilot/internal/aws"
	"github.com/kent/infrapilot/internal/config"
	"github.com/kent/infrapilot/internal/k8s"
	"github.com/kent/infrapilot/internal/store"
	"github.com/kent/infrapilot/internal/terraform"
)

// Registry holds dependencies for MCP tool handlers.
type Registry struct {
	Config *config.Config
	Store  *store.Store
}

// NewRegistry creates a tool registry.
func NewRegistry(cfg *config.Config, st *store.Store) *Registry {
	return &Registry{Config: cfg, Store: st}
}

func (r *Registry) awsClient(ctx context.Context) (*aws.Client, error) {
	return aws.NewClient(ctx, r.Config.AWSRegion)
}

func (r *Registry) k8sClient(contextName string) (*k8s.Client, error) {
	return k8s.NewClient(r.Config.Kubeconfig, contextName)
}

// ListResources discovers AWS infrastructure.
func (r *Registry) ListResources(ctx context.Context) (string, error) {
	client, err := r.awsClient(ctx)
	if err != nil {
		return "", err
	}
	inv, err := client.Discover(ctx)
	if err != nil {
		return "", err
	}
	if r.Store != nil {
		if json, err := inv.ToJSON(); err == nil {
			_ = r.Store.SaveSnapshot(ctx, "aws_inventory", json)
		}
	}
	return inv.ToJSON()
}

// ExplainArchitecture returns a human-readable architecture summary.
func (r *Registry) ExplainArchitecture(ctx context.Context) (string, error) {
	client, err := r.awsClient(ctx)
	if err != nil {
		return "", err
	}
	inv, err := client.Discover(ctx)
	if err != nil {
		return "", err
	}
	return inv.ArchitectureExplanation(), nil
}

// TerraformInventory parses Terraform state.
func (r *Registry) TerraformInventory() (string, error) {
	inv, err := terraform.ParseState(r.Config.TerraformState)
	if err != nil {
		return "", err
	}
	return inv.Summary(), nil
}

// SecurityScan runs AWS security checks.
func (r *Registry) SecurityScan(ctx context.Context) (string, error) {
	client, err := r.awsClient(ctx)
	if err != nil {
		return "", err
	}
	report, err := client.ScanSecurity(ctx)
	if err != nil {
		return "", err
	}
	if r.Store != nil {
		if json, err := report.ToJSON(); err == nil {
			_ = r.Store.SaveScan(ctx, "security_scan", json)
		}
	}
	return formatSecurityReport(report), nil
}

// CheckPublicAccess finds publicly exposed resources.
func (r *Registry) CheckPublicAccess(ctx context.Context) (string, error) {
	client, err := r.awsClient(ctx)
	if err != nil {
		return "", err
	}
	report, err := client.CheckPublicAccess(ctx)
	if err != nil {
		return "", err
	}
	return formatSecurityReport(report), nil
}

// IAMReview reviews IAM posture.
func (r *Registry) IAMReview(ctx context.Context) (string, error) {
	client, err := r.awsClient(ctx)
	if err != nil {
		return "", err
	}
	review, err := client.ReviewIAM(ctx)
	if err != nil {
		return "", err
	}
	json, err := review.ToJSON()
	if err != nil {
		return "", err
	}
	return json, nil
}

// CostAnalysis returns cost optimization summary.
func (r *Registry) CostAnalysis(ctx context.Context) (string, error) {
	client, err := r.awsClient(ctx)
	if err != nil {
		return "", err
	}
	return client.CostAnalysis(ctx)
}

// ResourceWasteDetection finds wasteful resources.
func (r *Registry) ResourceWasteDetection(ctx context.Context) (string, error) {
	client, err := r.awsClient(ctx)
	if err != nil {
		return "", err
	}
	report, err := client.DetectWaste(ctx)
	if err != nil {
		return "", err
	}
	if r.Store != nil {
		if json, err := report.ToJSON(); err == nil {
			_ = r.Store.SaveScan(ctx, "cost_waste", json)
		}
	}
	return report.ToJSON()
}

// SavingsReport returns formatted savings report.
func (r *Registry) SavingsReport(ctx context.Context) (string, error) {
	client, err := r.awsClient(ctx)
	if err != nil {
		return "", err
	}
	report, err := client.DetectWaste(ctx)
	if err != nil {
		return "", err
	}
	return report.SavingsReport(), nil
}

// ClusterHealth checks Kubernetes cluster health.
func (r *Registry) ClusterHealth(ctx context.Context, kubeContext string) (string, error) {
	client, err := r.k8sClient(kubeContext)
	if err != nil {
		return "", err
	}
	report, err := client.ClusterHealth(ctx)
	if err != nil {
		return "", err
	}
	return report.Summary() + "\n\n" + mustJSON(report), nil
}

// DeploymentStatus lists deployment availability.
func (r *Registry) DeploymentStatus(ctx context.Context, namespace, kubeContext string) (string, error) {
	client, err := r.k8sClient(kubeContext)
	if err != nil {
		return "", err
	}
	statuses, err := client.ListDeploymentStatus(ctx, namespace)
	if err != nil {
		return "", err
	}
	return mustJSON(statuses), nil
}

// PodDiagnostics returns diagnostics for a specific pod.
func (r *Registry) PodDiagnostics(ctx context.Context, namespace, name, kubeContext string) (string, error) {
	client, err := r.k8sClient(kubeContext)
	if err != nil {
		return "", err
	}
	return client.PodDiagnostics(ctx, namespace, name)
}

// IncidentReport correlates recent infrastructure signals into an incident summary.
func (r *Registry) IncidentReport(ctx context.Context) (string, error) {
	var sections []string

	client, err := r.awsClient(ctx)
	if err == nil {
		if inv, err := client.Discover(ctx); err == nil {
			sections = append(sections, fmt.Sprintf("AWS (%s): %d EC2, %d EKS, %d RDS",
				inv.Summary.Region, inv.Summary.EC2Instances, inv.Summary.EKSClusters, inv.Summary.RDSInstances))
		}
		if sec, err := client.ScanSecurity(ctx); err == nil && len(sec.Findings) > 0 {
			sections = append(sections, fmt.Sprintf("Security: %d open findings (highest severity: %s)",
				len(sec.Findings), highestSeverity(sec.Findings)))
		}
	}

	if kc, err := r.k8sClient(""); err == nil {
		if health, err := kc.ClusterHealth(ctx); err == nil {
			if len(health.CrashLoopPods) > 0 || len(health.PendingPods) > 0 {
				sections = append(sections, fmt.Sprintf("Kubernetes: %d crash loops, %d pending pods",
					len(health.CrashLoopPods), len(health.PendingPods)))
			}
		}
	}

	if len(sections) == 0 {
		return "No significant incident signals detected in current infrastructure scan.", nil
	}

	return "Incident Investigation Summary\n\n" + joinLines(sections), nil
}

// InfrastructureSummary generates a cross-cutting infrastructure summary.
func (r *Registry) InfrastructureSummary(ctx context.Context) (string, error) {
	var parts []string

	arch, err := r.ExplainArchitecture(ctx)
	if err == nil {
		parts = append(parts, arch)
	} else {
		parts = append(parts, fmt.Sprintf("AWS: unavailable (%v)", err))
	}

	if r.Config.TerraformState != "" {
		if tf, err := r.TerraformInventory(); err == nil {
			parts = append(parts, "\n"+tf)
		}
	}

	if kc, err := r.k8sClient(""); err == nil {
		if health, err := kc.ClusterHealth(ctx); err == nil {
			parts = append(parts, "\n"+health.Summary())
		}
	}

	return joinLines(parts), nil
}

// EnvironmentAudit runs security, cost, and health checks.
func (r *Registry) EnvironmentAudit(ctx context.Context) (string, error) {
	var sections []string

	if sec, err := r.SecurityScan(ctx); err == nil {
		sections = append(sections, "=== Security ===\n"+sec)
	}
	if cost, err := r.SavingsReport(ctx); err == nil {
		sections = append(sections, "=== Cost ===\n"+cost)
	}
	if health, err := r.ClusterHealth(ctx, ""); err == nil {
		sections = append(sections, "=== Kubernetes ===\n"+health)
	} else {
		sections = append(sections, fmt.Sprintf("=== Kubernetes ===\nunavailable: %v", err))
	}

	return joinLines(sections), nil
}

func formatSecurityReport(report *aws.SecurityReport) string {
	if len(report.Findings) == 0 {
		return fmt.Sprintf("Security Scan (Region %s): No issues found.", report.Region)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Security Scan (Region %s): %d findings\n\n", report.Region, len(report.Findings))
	for _, f := range report.Findings {
		fmt.Fprintf(&b, "[%s] %s - %s: %s\n", f.Severity, f.Category, f.Resource, f.Description)
	}
	return b.String()
}

func highestSeverity(findings []aws.Finding) string {
	order := map[string]int{"critical": 4, "high": 3, "medium": 2, "low": 1}
	best := "low"
	bestScore := 0
	for _, f := range findings {
		if score := order[f.Severity]; score > bestScore {
			bestScore = score
			best = f.Severity
		}
	}
	return best
}

func mustJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return string(data)
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n\n")
}
