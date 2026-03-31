package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vitistack/common/pkg/v1alpha1"
	"github.com/vitistack/viti-mcp/internal/k8s"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func registerNetworkTools(s *server.MCPServer, m *k8s.Manager) {
	s.AddTool(
		mcp.NewTool("list_network_namespaces",
			mcp.WithDescription("List NetworkNamespace resources. Shows VLAN IDs, IP prefixes, and associated clusters."),
			mcp.WithString("zone", mcp.Description("Filter by availability zone name.")),
			mcp.WithString("namespace", mcp.Description("Filter by Kubernetes namespace.")),
		),
		listNetworkNamespacesHandler(m),
	)

	s.AddTool(
		mcp.NewTool("get_network_namespace",
			mcp.WithDescription("Get detailed information about a specific NetworkNamespace including IP prefixes, VLAN, egress IPs, and associated clusters."),
			mcp.WithString("zone", mcp.Required(), mcp.Description("The availability zone to query.")),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("The Kubernetes namespace.")),
			mcp.WithString("name", mcp.Required(), mcp.Description("The name of the NetworkNamespace resource.")),
		),
		getNetworkNamespaceHandler(m),
	)

	s.AddTool(
		mcp.NewTool("list_network_configurations",
			mcp.WithDescription("List NetworkConfiguration resources. Shows network interface details including IPs, VLANs, and gateways."),
			mcp.WithString("zone", mcp.Description("Filter by availability zone name.")),
			mcp.WithString("namespace", mcp.Description("Filter by Kubernetes namespace.")),
		),
		listNetworkConfigurationsHandler(m),
	)
}

func listNetworkNamespacesHandler(m *k8s.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		zone := req.GetString("zone", "")
		ns := req.GetString("namespace", "")

		results := k8s.QueryAll(ctx, m, zone, func(ctx context.Context, zc *k8s.ZoneClient) ([]networkNamespaceSummary, error) {
			var list v1alpha1.NetworkNamespaceList
			opts := []k8sclient.ListOption{}
			if ns != "" {
				opts = append(opts, k8sclient.InNamespace(ns))
			}
			if err := zc.Client.List(ctx, &list, opts...); err != nil {
				return nil, err
			}

			var summaries []networkNamespaceSummary
			for _, nn := range list.Items {
				summaries = append(summaries, toNetworkNamespaceSummary(zc.Name, &nn))
			}
			return summaries, nil
		})

		return formatQueryResults("network_namespaces", results)
	}
}

func getNetworkNamespaceHandler(m *k8s.Manager) server.ToolHandlerFunc {
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

		var nn v1alpha1.NetworkNamespace
		key := k8sclient.ObjectKey{Namespace: namespace, Name: name}
		if err := zc.Client.Get(ctx, key, &nn); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting network namespace %s/%s: %v", namespace, name, err)), nil
		}

		detail := networkNamespaceDetail{
			networkNamespaceSummary: toNetworkNamespaceSummary(zc.Name, &nn),
			DatacenterID:            nn.Status.DataCenterIdentifier,
			SupervisorID:            nn.Status.SupervisorIdentifier,
			NamespaceID:             nn.Status.NamespaceID,
			IPv4EgressIP:            nn.Status.IPv4EgressIP,
			IPv6EgressIP:            nn.Status.IPv6EgressIP,
			AssociatedClusters:      nn.Status.AssociatedKubernetesClusterIDs,
		}
		return formatResult(detail)
	}
}

func listNetworkConfigurationsHandler(m *k8s.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		zone := req.GetString("zone", "")
		ns := req.GetString("namespace", "")

		results := k8s.QueryAll(ctx, m, zone, func(ctx context.Context, zc *k8s.ZoneClient) ([]networkConfigSummary, error) {
			var list v1alpha1.NetworkConfigurationList
			opts := []k8sclient.ListOption{}
			if ns != "" {
				opts = append(opts, k8sclient.InNamespace(ns))
			}
			if err := zc.Client.List(ctx, &list, opts...); err != nil {
				return nil, err
			}

			var summaries []networkConfigSummary
			for _, nc := range list.Items {
				ifaceCount := len(nc.Spec.NetworkInterfaces)
				summaries = append(summaries, networkConfigSummary{
					Zone:           zc.Name,
					Namespace:      nc.Namespace,
					Name:           nc.Name,
					Phase:          nc.Status.Phase,
					InterfaceCount: ifaceCount,
				})
			}
			return summaries, nil
		})

		return formatQueryResults("network_configurations", results)
	}
}

type networkNamespaceSummary struct {
	Zone         string `json:"zone"`
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Phase        string `json:"phase,omitempty"`
	VlanID       int    `json:"vlanId,omitempty"`
	IPv4Prefix   string `json:"ipv4Prefix,omitempty"`
	IPv6Prefix   string `json:"ipv6Prefix,omitempty"`
	ClusterCount int    `json:"associatedClusterCount"`
}

type networkNamespaceDetail struct {
	networkNamespaceSummary
	DatacenterID       string   `json:"datacenterIdentifier,omitempty"`
	SupervisorID       string   `json:"supervisorIdentifier,omitempty"`
	NamespaceID        string   `json:"namespaceId,omitempty"`
	IPv4EgressIP       string   `json:"ipv4EgressIP,omitempty"`
	IPv6EgressIP       string   `json:"ipv6EgressIP,omitempty"`
	AssociatedClusters []string `json:"associatedClusters,omitempty"`
}

type networkConfigSummary struct {
	Zone           string `json:"zone"`
	Namespace      string `json:"namespace"`
	Name           string `json:"name"`
	Phase          string `json:"phase,omitempty"`
	InterfaceCount int    `json:"interfaceCount"`
}

func toNetworkNamespaceSummary(zoneName string, nn *v1alpha1.NetworkNamespace) networkNamespaceSummary {
	return networkNamespaceSummary{
		Zone:         zoneName,
		Namespace:    nn.Namespace,
		Name:         nn.Name,
		Phase:        nn.Status.Phase,
		VlanID:       nn.Status.VlanID,
		IPv4Prefix:   nn.Status.IPv4Prefix,
		IPv6Prefix:   nn.Status.IPv6Prefix,
		ClusterCount: len(nn.Status.AssociatedKubernetesClusterIDs),
	}
}
