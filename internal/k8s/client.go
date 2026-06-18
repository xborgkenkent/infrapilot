package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps Kubernetes API access.
type Client struct {
	Clientset kubernetes.Interface
	Context   string
}

// NewClient builds a client from kubeconfig path and optional context.
func NewClient(kubeconfig, contextName string) (*Client, error) {
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}

	ctx := contextName
	if ctx == "" {
		raw, err := loadingRules.Load()
		if err == nil {
			ctx = raw.CurrentContext
		}
	}

	return &Client{Clientset: cs, Context: ctx}, nil
}

// PodIssue describes an unhealthy pod.
type PodIssue struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Phase     string `json:"phase"`
	Reason    string `json:"reason"`
	Message   string `json:"message,omitempty"`
}

// ClusterHealthReport summarizes cluster health.
type ClusterHealthReport struct {
	Context           string   `json:"context"`
	TotalNodes        int      `json:"total_nodes"`
	ReadyNodes        int      `json:"ready_nodes"`
	CrashLoopPods     []PodIssue `json:"crash_loop_pods"`
	PendingPods       []PodIssue `json:"pending_pods"`
	NodePressure      []string `json:"node_pressure"`
}

// ClusterHealth inspects cluster health.
func (c *Client) ClusterHealth(ctx context.Context) (*ClusterHealthReport, error) {
	report := &ClusterHealthReport{Context: c.Context}

	nodes, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	report.TotalNodes = len(nodes.Items)
	for _, node := range nodes.Items {
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				report.ReadyNodes++
			}
			if cond.Status == corev1.ConditionTrue &&
				(cond.Type == corev1.NodeMemoryPressure || cond.Type == corev1.NodeDiskPressure) {
				report.NodePressure = append(report.NodePressure, fmt.Sprintf("%s: %s", node.Name, cond.Type))
			}
		}
	}

	pods, err := c.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodPending {
			report.PendingPods = append(report.PendingPods, PodIssue{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				Phase:     string(pod.Status.Phase),
				Reason:    pod.Status.Reason,
				Message:   pod.Status.Message,
			})
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				report.CrashLoopPods = append(report.CrashLoopPods, PodIssue{
					Namespace: pod.Namespace,
					Name:      pod.Name,
					Phase:     string(pod.Status.Phase),
					Reason:    cs.State.Waiting.Reason,
					Message:   cs.State.Waiting.Message,
				})
			}
		}
	}

	return report, nil
}

// DeploymentStatus describes deployment availability.
type DeploymentStatus struct {
	Namespace      string `json:"namespace"`
	Name           string `json:"name"`
	Replicas       int32 `json:"replicas"`
	ReadyReplicas  int32 `json:"ready_replicas"`
	Available      bool  `json:"available"`
}

// ListDeploymentStatus returns deployment availability.
func (c *Client) ListDeploymentStatus(ctx context.Context, namespace string) ([]DeploymentStatus, error) {
	deps, err := c.Clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}

	var statuses []DeploymentStatus
	for _, d := range deps.Items {
		ready := d.Status.ReadyReplicas
		desired := d.Status.Replicas
		if desired == 0 && d.Spec.Replicas != nil {
			desired = *d.Spec.Replicas
		}
		statuses = append(statuses, DeploymentStatus{
			Namespace:     d.Namespace,
			Name:          d.Name,
			Replicas:      desired,
			ReadyReplicas: ready,
			Available:     ready == desired && desired > 0,
		})
	}
	return statuses, nil
}

// PodDiagnostics returns detailed diagnostics for a pod.
func (c *Client) PodDiagnostics(ctx context.Context, namespace, name string) (string, error) {
	pod, err := c.Clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get pod: %w", err)
	}

	events, err := c.Clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", name),
	})
	if err != nil {
		return "", fmt.Errorf("list events: %w", err)
	}

	var report strings.Builder
	fmt.Fprintf(&report, "Pod: %s/%s\n", namespace, name)
	fmt.Fprintf(&report, "Phase: %s\n", pod.Status.Phase)
	fmt.Fprintf(&report, "Node: %s\n", pod.Spec.NodeName)
	fmt.Fprintf(&report, "\nContainers:\n")
	for _, cs := range pod.Status.ContainerStatuses {
		fmt.Fprintf(&report, "- %s ready=%t restarts=%d\n", cs.Name, cs.Ready, cs.RestartCount)
		if cs.State.Waiting != nil {
			fmt.Fprintf(&report, "  waiting: %s - %s\n", cs.State.Waiting.Reason, cs.State.Waiting.Message)
		}
		if cs.LastTerminationState.Terminated != nil {
			t := cs.LastTerminationState.Terminated
			fmt.Fprintf(&report, "  last terminated: exit=%d reason=%s\n", t.ExitCode, t.Reason)
		}
	}
	fmt.Fprintf(&report, "\nRecent Events:\n")
	for _, e := range events.Items {
		fmt.Fprintf(&report, "- %s: %s\n", e.Reason, e.Message)
	}
	return report.String(), nil
}

func (r *ClusterHealthReport) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *ClusterHealthReport) Summary() string {
	return fmt.Sprintf(`Cluster Health (%s):

- Nodes: %d ready / %d total
- CrashLoopBackOff pods: %d
- Pending pods: %d
- Node pressure signals: %d`,
		r.Context, r.ReadyNodes, r.TotalNodes,
		len(r.CrashLoopPods), len(r.PendingPods), len(r.NodePressure))
}
