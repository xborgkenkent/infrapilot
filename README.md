# InfraPilot MCP — AI Infrastructure Copilot

## Overview

InfraPilot MCP is a local-first Model Context Protocol (MCP) server that enables AI assistants to understand, analyze, and interact with real cloud infrastructure environments. Instead of providing generic DevOps advice, InfraPilot gives AI direct visibility into AWS resources, Kubernetes clusters, Terraform state, and operational metrics, allowing it to answer questions using actual infrastructure data.

The goal is to transform AI assistants into intelligent DevOps and Cloud Engineering copilots capable of performing architecture analysis, security reviews, cost optimization assessments, operational troubleshooting, and infrastructure documentation.

Designed as a local-first application, InfraPilot prioritizes security by running entirely within the user's environment and using existing cloud credentials and tooling.

---

## Problem Statement

Modern cloud environments are becoming increasingly complex. Organizations often manage:

* Hundreds of cloud resources
* Multiple Kubernetes clusters
* Large Terraform codebases
* Various monitoring and alerting systems
* Growing infrastructure costs

While AI can explain cloud concepts and best practices, it typically lacks awareness of a company's actual environment.

Questions such as:

* Why is our AWS bill increasing?
* What security risks exist in our infrastructure?
* Which resources are unused?
* Why is Kubernetes unhealthy?
* What changed before the last incident?

cannot be answered accurately without access to infrastructure data.

InfraPilot bridges this gap by providing real-time infrastructure context to AI assistants through MCP.

---

## Key Features

### Infrastructure Discovery

Automatically discover and inventory:

* AWS Resources
* VPCs
* EC2 Instances
* EKS Clusters
* RDS Databases
* S3 Buckets
* Load Balancers
* Security Groups

Example:

```text
User:
Explain my infrastructure.

AI:
Your environment contains:

- 1 VPC
- 2 EKS clusters
- 3 RDS instances
- 12 EC2 instances
- 5 S3 buckets

Traffic Flow:
CloudFront → ALB → EKS → Aurora PostgreSQL
```

---

### Security Analysis

Identify security risks and misconfigurations.

Checks include:

* Publicly accessible S3 buckets
* Security groups exposing sensitive ports
* IAM users without MFA
* Public EKS API endpoints
* Unencrypted resources
* Excessive IAM permissions

Example:

```text
Critical Issues:

- EKS API endpoint exposed publicly
- 8 IAM users without MFA
- S3 bucket allows anonymous access
```

---

### Cost Optimization

Analyze cloud spending and identify waste.

Detect:

* Idle EC2 instances
* Unattached EBS volumes
* Unused Elastic IPs
* Overprovisioned EKS node groups
* Excessive NAT Gateway costs
* Stale snapshots

Example:

```text
Potential Savings:

- 4 unattached EBS volumes
- 2 idle EC2 instances
- 1 unused NAT Gateway

Estimated Monthly Savings:
$420/month
```

---

### Kubernetes Health Monitoring

Inspect Kubernetes clusters and workloads.

Checks:

* CrashLoopBackOff pods
* Pending workloads
* Failed deployments
* Node pressure conditions
* Resource utilization

Example:

```text
Cluster Health:

- 2 Pods in CrashLoopBackOff
- 1 Deployment unavailable
- Node memory pressure detected
```

---

### Terraform Intelligence

Analyze Terraform state and infrastructure definitions.

Capabilities:

* Resource inventory
* Drift detection
* Infrastructure summaries
* Dependency mapping

Example:

```text
Terraform Resources:

- aws_vpc.main
- aws_eks_cluster.production
- aws_rds_cluster.database

Total Managed Resources:
43
```

---

### Incident Investigation

Correlate infrastructure events and operational data to generate incident summaries.

Example:

```text
Incident Summary

Time:
15:03 UTC

Root Cause:
Deployment introduced memory leak.

Impact:
API latency increased by 300%.

Resolution:
Rollback completed at 15:21 UTC.
```

---

### Infrastructure Documentation

Generate architecture documentation automatically.

Outputs:

* Infrastructure summaries
* Service relationships
* Deployment diagrams
* Operational runbooks

This helps maintain up-to-date documentation without manual effort.

---

## MCP Tools

### Infrastructure

```text
list_resources
explain_architecture
terraform_inventory
```

### Security

```text
security_scan
check_public_access
iam_review
```

### Cost

```text
cost_analysis
resource_waste_detection
savings_report
```

### Kubernetes

```text
cluster_health
deployment_status
pod_diagnostics
```

### Operations

```text
incident_report
infrastructure_summary
environment_audit
```

---

## Technology Stack

### Backend

* Go 1.24+
* MCP Go SDK
* REST APIs
* JSON Processing

### Cloud Integrations

* AWS SDK for Go v2
* Terraform State Parsing
* Kubernetes Client-Go

### Data Storage

* SQLite (Local Metadata)
* JSON Configuration Files

### Infrastructure Access

* AWS CLI
* kubectl
* Terraform CLI

### AI Integration

Compatible with:

* Claude Desktop
* ChatGPT MCP
* Cursor
* Windsurf
* Other MCP-compatible clients

---

## Architecture

```text
+-------------------+
| AI Assistant      |
| Claude / ChatGPT  |
+---------+---------+
          |
          |
          v
+-------------------+
| InfraPilot MCP    |
+-------------------+
          |
    +-----+-----+
    |     |     |
    v     v     v
 AWS   Terraform K8s
 CLI    State    API
```

---

## Security Model

InfraPilot is designed as a local-first application.

Security principles:

* Runs entirely on local machines
* No cloud-hosted backend
* Uses existing cloud credentials
* Read-only access by default
* No infrastructure modifications without explicit approval
* Sensitive data remains within the user's environment

---

## Future Enhancements

### Multi-Cloud Support

* AWS
* Azure
* Google Cloud Platform

### Monitoring Integrations

* Datadog
* Grafana
* Prometheus
* New Relic

### Incident Management

* PagerDuty
* Opsgenie
* ServiceNow

### AI-Powered Recommendations

* Terraform change generation
* Security remediation suggestions
* Automated architecture reviews
* Capacity planning insights

---

## Target Audience

* DevOps Engineers
* Platform Engineers
* Cloud Engineers
* Site Reliability Engineers (SREs)
* Infrastructure Architects
* Small and Medium Engineering Teams
* Managed Service Providers (MSPs)

---

## Project Goal

Build an AI-powered infrastructure copilot that enables engineers to understand, troubleshoot, secure, and optimize cloud environments through natural language conversations backed by real infrastructure data rather than generic AI knowledge.
