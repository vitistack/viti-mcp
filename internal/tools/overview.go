package tools

import (
	"context"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vitistack/common/pkg/v1alpha1"
	"github.com/vitistack/viti-mcp/internal/k8s"
)

func registerOverviewTools(s *server.MCPServer, m *k8s.Manager) {
	s.AddTool(
		mcp.NewTool("list_zones",
			mcp.WithDescription("List all configured availability zones and their connectivity status."),
		),
		listZonesHandler(m),
	)

	s.AddTool(
		mcp.NewTool("infrastructure_overview",
			mcp.WithDescription("Get a high-level overview of the entire infrastructure across all availability zones. Shows total clusters, machines, providers, and resource usage per zone."),
		),
		infrastructureOverviewHandler(m),
	)
}

func listZonesHandler(m *k8s.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var zones []zoneInfo
		for _, zc := range m.All() {
			info := zoneInfo{
				Name:   zc.Name,
				Region: zc.Region,
			}

			// Quick connectivity check: try listing vitistack resources
			var list v1alpha1.VitistackList
			if err := zc.Client.List(ctx, &list); err != nil {
				info.Connected = false
				info.Error = err.Error()
			} else {
				info.Connected = true
			}

			zones = append(zones, info)
		}
		return formatResult(zones)
	}
}

func infrastructureOverviewHandler(m *k8s.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		allZones := m.All()
		overviews := make([]zoneOverview, len(allZones))

		var wg sync.WaitGroup
		for i, zc := range allZones {
			wg.Add(1)
			go func(idx int, zc *k8s.ZoneClient) {
				defer wg.Done()
				overviews[idx] = gatherOverview(ctx, zc)
			}(i, zc)
		}
		wg.Wait()

		total := infrastructureSummary{
			Zones: overviews,
		}
		for _, o := range overviews {
			total.TotalClusters += o.Clusters
			total.TotalMachines += o.Machines
			total.TotalMachineProviders += o.MachineProviders
			total.TotalKubernetesProviders += o.KubernetesProviders
		}

		return formatResult(total)
	}
}

func gatherOverview(ctx context.Context, zc *k8s.ZoneClient) zoneOverview {
	o := zoneOverview{
		Zone:   zc.Name,
		Region: zc.Region,
	}

	var clusters v1alpha1.KubernetesClusterList
	if err := zc.Client.List(ctx, &clusters); err != nil {
		o.Error = err.Error()
		return o
	}
	o.Clusters = len(clusters.Items)

	for _, c := range clusters.Items {
		switch c.Status.Phase {
		case "Running":
			o.ClustersRunning++
		case "Failed":
			o.ClustersFailed++
		}
	}

	var machines v1alpha1.MachineList
	if err := zc.Client.List(ctx, &machines); err == nil {
		o.Machines = len(machines.Items)
		for _, m := range machines.Items {
			switch m.Status.Phase {
			case "Running":
				o.MachinesRunning++
			case "Failed":
				o.MachinesFailed++
			}
		}
	}

	var mp v1alpha1.MachineProviderList
	if err := zc.Client.List(ctx, &mp); err == nil {
		o.MachineProviders = len(mp.Items)
	}

	var kp v1alpha1.KubernetesProviderList
	if err := zc.Client.List(ctx, &kp); err == nil {
		o.KubernetesProviders = len(kp.Items)
	}

	return o
}

type zoneInfo struct {
	Name      string `json:"name"`
	Region    string `json:"region,omitempty"`
	Connected bool   `json:"connected"`
	Error     string `json:"error,omitempty"`
}

type zoneOverview struct {
	Zone                string `json:"zone"`
	Region              string `json:"region,omitempty"`
	Clusters            int    `json:"clusters"`
	ClustersRunning     int    `json:"clustersRunning"`
	ClustersFailed      int    `json:"clustersFailed"`
	Machines            int    `json:"machines"`
	MachinesRunning     int    `json:"machinesRunning"`
	MachinesFailed      int    `json:"machinesFailed"`
	MachineProviders    int    `json:"machineProviders"`
	KubernetesProviders int    `json:"kubernetesProviders"`
	Error               string `json:"error,omitempty"`
}

type infrastructureSummary struct {
	TotalClusters            int            `json:"totalClusters"`
	TotalMachines            int            `json:"totalMachines"`
	TotalMachineProviders    int            `json:"totalMachineProviders"`
	TotalKubernetesProviders int            `json:"totalKubernetesProviders"`
	Zones                    []zoneOverview `json:"zones"`
}
