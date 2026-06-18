package server

import (
	"context"
	"fmt"

	"github.com/kent/infrapilot/internal/config"
	"github.com/kent/infrapilot/internal/store"
	"github.com/kent/infrapilot/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// New creates and configures the InfraPilot MCP server.
func New(cfg *config.Config, st *store.Store) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "infrapilot",
		Version: "0.1.0",
	}, nil)

	reg := tools.NewRegistry(cfg, st)

	registerInfrastructureTools(server, reg)
	registerSecurityTools(server, reg)
	registerCostTools(server, reg)
	registerKubernetesTools(server, reg)
	registerOperationsTools(server, reg)

	return server
}

func registerInfrastructureTools(server *mcp.Server, reg *tools.Registry) {
	type empty struct{}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_resources",
		Description: "Discover and inventory AWS infrastructure resources (VPCs, EC2, EKS, RDS, S3, load balancers, security groups)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return textResult(reg.ListResources(ctx))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "explain_architecture",
		Description: "Generate a human-readable summary of the infrastructure architecture and traffic flow",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return textResult(reg.ExplainArchitecture(ctx))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "terraform_inventory",
		Description: "Parse Terraform state and return a resource inventory summary",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return textResult(reg.TerraformInventory())
	})
}

func registerSecurityTools(server *mcp.Server, reg *tools.Registry) {
	type empty struct{}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "security_scan",
		Description: "Run security checks for S3 public access, open security groups, IAM MFA, public EKS endpoints, and unencrypted RDS",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return textResult(reg.SecurityScan(ctx))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "check_public_access",
		Description: "Identify publicly accessible S3 buckets, security groups, and RDS instances",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return textResult(reg.CheckPublicAccess(ctx))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "iam_review",
		Description: "Review IAM users for MFA compliance and excessive admin permissions",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return textResult(reg.IAMReview(ctx))
	})
}

func registerCostTools(server *mcp.Server, reg *tools.Registry) {
	type empty struct{}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cost_analysis",
		Description: "Analyze cloud spending patterns and identify cost optimization opportunities",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return textResult(reg.CostAnalysis(ctx))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "resource_waste_detection",
		Description: "Detect idle EC2 instances, unattached EBS volumes, unused Elastic IPs, and stale snapshots",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return textResult(reg.ResourceWasteDetection(ctx))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "savings_report",
		Description: "Generate a formatted report of potential monthly cost savings",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return textResult(reg.SavingsReport(ctx))
	})
}

func registerKubernetesTools(server *mcp.Server, reg *tools.Registry) {
	type clusterHealthArgs struct {
		Context string `json:"context" jsonschema:"Kubernetes context name (optional, uses current context if empty)"`
	}
	type deploymentStatusArgs struct {
		Namespace string `json:"namespace" jsonschema:"Kubernetes namespace (empty for all namespaces)"`
		Context   string `json:"context" jsonschema:"Kubernetes context name (optional)"`
	}
	type podDiagnosticsArgs struct {
		Namespace string `json:"namespace" jsonschema:"Pod namespace"`
		Name      string `json:"name" jsonschema:"Pod name"`
		Context   string `json:"context" jsonschema:"Kubernetes context name (optional)"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cluster_health",
		Description: "Inspect Kubernetes cluster health including CrashLoopBackOff pods, pending workloads, and node pressure",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args clusterHealthArgs) (*mcp.CallToolResult, any, error) {
		return textResult(reg.ClusterHealth(ctx, args.Context))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "deployment_status",
		Description: "Check deployment availability and replica readiness",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args deploymentStatusArgs) (*mcp.CallToolResult, any, error) {
		return textResult(reg.DeploymentStatus(ctx, args.Namespace, args.Context))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "pod_diagnostics",
		Description: "Get detailed diagnostics for a specific pod including container states and events",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args podDiagnosticsArgs) (*mcp.CallToolResult, any, error) {
		if args.Namespace == "" || args.Name == "" {
			return errorResult("namespace and name are required")
		}
		return textResult(reg.PodDiagnostics(ctx, args.Namespace, args.Name, args.Context))
	})
}

func registerOperationsTools(server *mcp.Server, reg *tools.Registry) {
	type empty struct{}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "incident_report",
		Description: "Correlate infrastructure signals to generate an incident investigation summary",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return textResult(reg.IncidentReport(ctx))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "infrastructure_summary",
		Description: "Generate a comprehensive infrastructure summary across AWS, Terraform, and Kubernetes",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return textResult(reg.InfrastructureSummary(ctx))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "environment_audit",
		Description: "Run a full environment audit covering security, cost, and Kubernetes health",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return textResult(reg.EnvironmentAudit(ctx))
	})
}

func textResult(text string, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return errorResult(err.Error())
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}

func errorResult(msg string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %s", msg)}},
		IsError: true,
	}, nil, nil
}
