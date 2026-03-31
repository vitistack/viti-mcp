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

func registerProviderTools(s *server.MCPServer, m *k8s.Manager) {
	s.AddTool(
		mcp.NewTool("list_machine_providers",
			mcp.WithDescription("List MachineProvider resources (infrastructure providers like Proxmox, KubeVirt, Azure). An availability zone can have multiple machine providers. Shows provider type, health, quota usage, and active machines."),
			mcp.WithString("zone", mcp.Description("Filter by availability zone name.")),
		),
		listMachineProvidersHandler(m),
	)

	s.AddTool(
		mcp.NewTool("get_machine_provider",
			mcp.WithDescription("Get detailed information about a specific MachineProvider including capabilities, quota, health status, and configuration."),
			mcp.WithString("zone", mcp.Required(), mcp.Description("The availability zone to query.")),
			mcp.WithString("name", mcp.Required(), mcp.Description("The name of the MachineProvider resource.")),
		),
		getMachineProviderHandler(m),
	)

	s.AddTool(
		mcp.NewTool("list_kubernetes_providers",
			mcp.WithDescription("List KubernetesProvider resources (e.g. Talos, AKS). An availability zone can have multiple kubernetes providers. Shows provider type, version, phase, and node counts."),
			mcp.WithString("zone", mcp.Description("Filter by availability zone name.")),
		),
		listKubernetesProvidersHandler(m),
	)
}

func listMachineProvidersHandler(m *k8s.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		zone := req.GetString("zone", "")

		results := k8s.QueryAll(ctx, m, zone, func(ctx context.Context, zc *k8s.ZoneClient) ([]machineProviderSummary, error) {
			var list v1alpha1.MachineProviderList
			if err := zc.Client.List(ctx, &list); err != nil {
				return nil, err
			}

			var summaries []machineProviderSummary
			for _, p := range list.Items {
				summaries = append(summaries, toMachineProviderSummary(zc.Name, &p))
			}
			return summaries, nil
		})

		return formatQueryResults("machine_providers", results)
	}
}

func getMachineProviderHandler(m *k8s.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		zoneName, err := req.RequireString("zone")
		if err != nil {
			return mcp.NewToolResultError("zone is required"), nil
		}
		name, err := req.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError("name is required"), nil
		}

		zc, ok := m.Get(zoneName)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("unknown availability zone: %s", zoneName)), nil
		}

		var provider v1alpha1.MachineProvider
		key := k8sclient.ObjectKey{Name: name}
		if err := zc.Client.Get(ctx, key, &provider); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting machine provider %s: %v", name, err)), nil
		}

		detail := toMachineProviderDetail(zc.Name, &provider)
		return formatResult(detail)
	}
}

func listKubernetesProvidersHandler(m *k8s.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		zone := req.GetString("zone", "")

		results := k8s.QueryAll(ctx, m, zone, func(ctx context.Context, zc *k8s.ZoneClient) ([]kubernetesProviderSummary, error) {
			var list v1alpha1.KubernetesProviderList
			if err := zc.Client.List(ctx, &list); err != nil {
				return nil, err
			}

			var summaries []kubernetesProviderSummary
			for _, p := range list.Items {
				summaries = append(summaries, kubernetesProviderSummary{
					Zone:        zc.Name,
					Name:        p.Name,
					DisplayName: p.Spec.DisplayName,
					Type:        p.Spec.ProviderType,
					Version:     p.Spec.Version,
					Region:      p.Spec.Region,
					Phase:       p.Status.Phase,
					NodeCount:   p.Status.NodeCount,
					ReadyNodes:  p.Status.ReadyNodeCount,
				})
			}
			return summaries, nil
		})

		return formatQueryResults("kubernetes_providers", results)
	}
}

type machineProviderSummary struct {
	Zone           string `json:"zone"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Type           string `json:"providerType"`
	Region         string `json:"region"`
	Phase          string `json:"phase"`
	HealthStatus   string `json:"healthStatus,omitempty"`
	ActiveMachines int    `json:"activeMachines"`
	CPUUsed        int    `json:"cpuUsed,omitempty"`
	CPUQuota       int    `json:"cpuQuota,omitempty"`
	MemoryUsedGB   int    `json:"memoryUsedGB,omitempty"`
	MemoryQuotaGB  int    `json:"memoryQuotaGB,omitempty"`
}

type machineProviderDetail struct {
	machineProviderSummary
	Zones        []string       `json:"zones,omitempty"`
	Endpoint     string         `json:"endpoint,omitempty"`
	Capabilities providerCaps   `json:"capabilities"`
	Quota        providerQuota  `json:"quota"`
	Health       providerHealth `json:"health"`
}

type providerCaps struct {
	MaxMachines   int  `json:"maxMachines,omitempty"`
	AutoScaling   bool `json:"autoScaling"`
	LoadBalancers bool `json:"loadBalancers"`
}

type providerQuota struct {
	CPUQuota       int `json:"cpuQuota"`
	CPUUsed        int `json:"cpuUsed"`
	MemoryQuotaGB  int `json:"memoryQuotaGB"`
	MemoryUsedGB   int `json:"memoryUsedGB"`
	StorageQuotaGB int `json:"storageQuotaGB"`
	StorageUsedGB  int `json:"storageUsedGB"`
	InstanceQuota  int `json:"instanceQuota"`
	InstanceUsed   int `json:"instanceUsed"`
}

type providerHealth struct {
	Status          string `json:"status"`
	APIConnectivity string `json:"apiConnectivity"`
	Authentication  string `json:"authentication"`
	LastCheck       string `json:"lastCheck,omitempty"`
	ResponseTimeMs  int    `json:"responseTimeMs,omitempty"`
}

type kubernetesProviderSummary struct {
	Zone        string `json:"zone"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Type        string `json:"providerType"`
	Version     string `json:"version"`
	Region      string `json:"region"`
	Phase       string `json:"phase"`
	NodeCount   int    `json:"nodeCount"`
	ReadyNodes  int    `json:"readyNodes"`
}

func toMachineProviderSummary(zoneName string, p *v1alpha1.MachineProvider) machineProviderSummary {
	s := machineProviderSummary{
		Zone:           zoneName,
		Name:           p.Name,
		DisplayName:    p.Spec.DisplayName,
		Type:           p.Spec.ProviderType,
		Region:         p.Spec.Region,
		Phase:          p.Status.Phase,
		ActiveMachines: p.Status.ActiveMachines,
	}
	if p.Status.Health.Status != "" {
		s.HealthStatus = p.Status.Health.Status
	}
	if p.Status.Quota.CPUQuota > 0 {
		s.CPUUsed = p.Status.Quota.CPUUsed
		s.CPUQuota = p.Status.Quota.CPUQuota
		s.MemoryUsedGB = p.Status.Quota.MemoryUsedGB
		s.MemoryQuotaGB = p.Status.Quota.MemoryQuotaGB
	}
	return s
}

func toMachineProviderDetail(zoneName string, p *v1alpha1.MachineProvider) machineProviderDetail {
	d := machineProviderDetail{
		machineProviderSummary: toMachineProviderSummary(zoneName, p),
		Zones:                  p.Spec.Zones,
	}

	if p.Spec.Endpoint.URL != "" {
		d.Endpoint = p.Spec.Endpoint.URL
	}

	d.Capabilities = providerCaps{
		MaxMachines:   p.Spec.Capabilities.MaxMachines,
		AutoScaling:   p.Spec.Capabilities.AutoScaling,
		LoadBalancers: p.Spec.Capabilities.LoadBalancers,
	}

	d.Quota = providerQuota{
		CPUQuota:       p.Status.Quota.CPUQuota,
		CPUUsed:        p.Status.Quota.CPUUsed,
		MemoryQuotaGB:  p.Status.Quota.MemoryQuotaGB,
		MemoryUsedGB:   p.Status.Quota.MemoryUsedGB,
		StorageQuotaGB: p.Status.Quota.StorageQuotaGB,
		StorageUsedGB:  p.Status.Quota.StorageUsedGB,
		InstanceQuota:  p.Status.Quota.InstanceQuota,
		InstanceUsed:   p.Status.Quota.InstanceUsed,
	}

	d.Health = providerHealth{
		Status:          p.Status.Health.Status,
		APIConnectivity: p.Status.Health.APIConnectivity,
		Authentication:  p.Status.Health.Authentication,
		ResponseTimeMs:  p.Status.Health.ResponseTimeMs,
	}

	if p.Status.Health.LastCheck != nil && !p.Status.Health.LastCheck.IsZero() {
		d.Health.LastCheck = p.Status.Health.LastCheck.String()
	}

	return d
}
