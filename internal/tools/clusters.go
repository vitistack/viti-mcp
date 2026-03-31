package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vitistack/common/pkg/v1alpha1"
	"github.com/vitistack/viti-mcp/internal/k8s"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func registerClusterTools(s *server.MCPServer, m *k8s.Manager) {
	s.AddTool(
		mcp.NewTool("list_clusters",
			mcp.WithDescription("List all KubernetesCluster resources across availability zones. Returns cluster name, namespace, phase, version, provider, and node counts."),
			mcp.WithString("zone", mcp.Description("Filter by availability zone name. Omit to query all zones.")),
			mcp.WithString("namespace", mcp.Description("Filter by Kubernetes namespace. Omit to list across all namespaces.")),
			mcp.WithString("phase", mcp.Description("Filter by cluster phase (e.g. Running, Provisioning, Failed).")),
		),
		listClustersHandler(m),
	)

	s.AddTool(
		mcp.NewTool("get_cluster",
			mcp.WithDescription("Get detailed information about a specific KubernetesCluster including topology, status, endpoints, and node pool details."),
			mcp.WithString("zone", mcp.Required(), mcp.Description("The availability zone to query.")),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("The namespace of the cluster.")),
			mcp.WithString("name", mcp.Required(), mcp.Description("The name of the KubernetesCluster resource.")),
		),
		getClusterHandler(m),
	)
}

func listClustersHandler(m *k8s.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		zone := req.GetString("zone", "")
		ns := req.GetString("namespace", "")
		phase := req.GetString("phase", "")

		results := k8s.QueryAll(ctx, m, zone, func(ctx context.Context, zc *k8s.ZoneClient) ([]clusterSummary, error) {
			var list v1alpha1.KubernetesClusterList
			opts := []k8sclient.ListOption{}
			if ns != "" {
				opts = append(opts, k8sclient.InNamespace(ns))
			}
			if err := zc.Client.List(ctx, &list, opts...); err != nil {
				return nil, err
			}

			var summaries []clusterSummary
			for _, c := range list.Items {
				if phase != "" && !strings.EqualFold(c.Status.Phase, phase) {
					continue
				}
				summaries = append(summaries, toClusterSummary(zc.Name, &c))
			}
			return summaries, nil
		})

		return formatQueryResults("clusters", results)
	}
}

func getClusterHandler(m *k8s.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		zoneName, err := req.RequireString("zone")
		if err != nil {
			return mcp.NewToolResultError("zone is required"), nil
		}
		namespace, err := req.RequireString("namespace")
		if err != nil {
			return mcp.NewToolResultError("namespace is required"), nil
		}
		name, err := req.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError("name is required"), nil
		}

		zc, ok := m.Get(zoneName)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("unknown availability zone: %s", zoneName)), nil
		}

		var cluster v1alpha1.KubernetesCluster
		key := k8sclient.ObjectKey{Namespace: namespace, Name: name}
		if err := zc.Client.Get(ctx, key, &cluster); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting cluster %s/%s: %v", namespace, name, err)), nil
		}

		detail := toClusterDetail(zc.Name, &cluster)
		return formatResult(detail)
	}
}

type clusterSummary struct {
	Zone         string `json:"zone"`
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Phase        string `json:"phase"`
	Version      string `json:"version"`
	Provider     string `json:"provider"`
	ControlPlane int    `json:"controlPlaneReplicas"`
	WorkerPools  int    `json:"workerPools"`
	TotalWorkers int    `json:"totalWorkerReplicas"`
	Environment  string `json:"environment,omitempty"`
	ClusterDC    string `json:"clusterDatacenter,omitempty"`
}

type clusterDetail struct {
	clusterSummary
	Topology clusterTopology `json:"topology"`
	Status   clusterStatus   `json:"status"`
}

type clusterTopology struct {
	ControlPlaneVersion      string           `json:"controlPlaneVersion"`
	ControlPlaneMachineClass string           `json:"controlPlaneMachineClass"`
	WorkerPools              []workerPoolInfo `json:"workerPools"`
}

type workerPoolInfo struct {
	Name         string `json:"name"`
	Replicas     int    `json:"replicas"`
	MachineClass string `json:"machineClass"`
	Version      string `json:"version,omitempty"`
}

type clusterStatus struct {
	Phase       string            `json:"phase"`
	EgressIP    string            `json:"egressIP,omitempty"`
	Endpoints   []clusterEndpoint `json:"endpoints,omitempty"`
	LastUpdated string            `json:"lastUpdated,omitempty"`
}

type clusterEndpoint struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

func toClusterSummary(zoneName string, c *v1alpha1.KubernetesCluster) clusterSummary {
	totalWorkers := 0
	for _, w := range c.Spec.Topology.Workers.NodePools {
		totalWorkers += w.Replicas
	}

	return clusterSummary{
		Zone:         zoneName,
		Namespace:    c.Namespace,
		Name:         c.Name,
		Phase:        c.Status.Phase,
		Version:      c.Spec.Topology.Version,
		Provider:     string(c.Spec.Cluster.Provider),
		ControlPlane: c.Spec.Topology.ControlPlane.Replicas,
		WorkerPools:  len(c.Spec.Topology.Workers.NodePools),
		TotalWorkers: totalWorkers,
		Environment:  c.Spec.Cluster.Environment,
		ClusterDC:    c.Spec.Cluster.Datacenter,
	}
}

func toClusterDetail(zoneName string, c *v1alpha1.KubernetesCluster) clusterDetail {
	var pools []workerPoolInfo
	for _, w := range c.Spec.Topology.Workers.NodePools {
		pools = append(pools, workerPoolInfo{
			Name:         w.Name,
			Replicas:     w.Replicas,
			MachineClass: w.MachineClass,
			Version:      w.Version,
		})
	}

	var endpoints []clusterEndpoint
	for _, ep := range c.Status.State.Endpoints {
		endpoints = append(endpoints, clusterEndpoint{
			Name:    ep.Name,
			Address: ep.Address,
		})
	}

	detail := clusterDetail{
		clusterSummary: toClusterSummary(zoneName, c),
		Topology: clusterTopology{
			ControlPlaneVersion:      c.Spec.Topology.ControlPlane.Version,
			ControlPlaneMachineClass: c.Spec.Topology.ControlPlane.MachineClass,
			WorkerPools:              pools,
		},
		Status: clusterStatus{
			Phase:     c.Status.Phase,
			EgressIP:  c.Status.State.EgressIP,
			Endpoints: endpoints,
		},
	}

	if !c.Status.State.LastUpdated.IsZero() {
		detail.Status.LastUpdated = c.Status.State.LastUpdated.String()
	}

	return detail
}
