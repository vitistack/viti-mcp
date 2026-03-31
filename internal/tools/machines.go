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

func registerMachineTools(s *server.MCPServer, m *k8s.Manager) {
	s.AddTool(
		mcp.NewTool("list_machines",
			mcp.WithDescription("List Machine resources across availability zones. Returns machine name, status, provider, IPs, CPU, and memory."),
			mcp.WithString("zone", mcp.Description("Filter by availability zone name.")),
			mcp.WithString("namespace", mcp.Description("Filter by Kubernetes namespace.")),
			mcp.WithString("phase", mcp.Description("Filter by machine phase (e.g. Running, Pending, Failed, Terminated).")),
			mcp.WithString("provider", mcp.Description("Filter by provider type (e.g. proxmox, kubevirt, aks).")),
		),
		listMachinesHandler(m),
	)

	s.AddTool(
		mcp.NewTool("get_machine",
			mcp.WithDescription("Get detailed information about a specific Machine including hardware specs, network interfaces, disk status, and conditions."),
			mcp.WithString("zone", mcp.Required(), mcp.Description("The availability zone to query.")),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("The namespace of the machine.")),
			mcp.WithString("name", mcp.Required(), mcp.Description("The name of the Machine resource.")),
		),
		getMachineHandler(m),
	)

	s.AddTool(
		mcp.NewTool("list_machine_classes",
			mcp.WithDescription("List available MachineClass resources (VM flavors/sizes). Returns CPU, memory, GPU specs, and supported providers."),
			mcp.WithString("zone", mcp.Description("Filter by availability zone name.")),
		),
		listMachineClassesHandler(m),
	)
}

func listMachinesHandler(m *k8s.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		zone := req.GetString("zone", "")
		ns := req.GetString("namespace", "")
		phase := req.GetString("phase", "")
		provider := req.GetString("provider", "")

		results := k8s.QueryAll(ctx, m, zone, func(ctx context.Context, zc *k8s.ZoneClient) ([]machineSummary, error) {
			var list v1alpha1.MachineList
			opts := []k8sclient.ListOption{}
			if ns != "" {
				opts = append(opts, k8sclient.InNamespace(ns))
			}
			if err := zc.Client.List(ctx, &list, opts...); err != nil {
				return nil, err
			}

			var summaries []machineSummary
			for _, m := range list.Items {
				if phase != "" && !strings.EqualFold(m.Status.Phase, phase) {
					continue
				}
				if provider != "" && !strings.EqualFold(string(m.Spec.Provider), provider) {
					continue
				}
				summaries = append(summaries, toMachineSummary(zc.Name, &m))
			}
			return summaries, nil
		})

		return formatQueryResults("machines", results)
	}
}

func getMachineHandler(m *k8s.Manager) server.ToolHandlerFunc {
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

		var machine v1alpha1.Machine
		key := k8sclient.ObjectKey{Namespace: namespace, Name: name}
		if err := zc.Client.Get(ctx, key, &machine); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting machine %s/%s: %v", namespace, name, err)), nil
		}

		detail := toMachineDetail(zc.Name, &machine)
		return formatResult(detail)
	}
}

func listMachineClassesHandler(m *k8s.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		zone := req.GetString("zone", "")

		results := k8s.QueryAll(ctx, m, zone, func(ctx context.Context, zc *k8s.ZoneClient) ([]machineClassSummary, error) {
			var list v1alpha1.MachineClassList
			if err := zc.Client.List(ctx, &list); err != nil {
				return nil, err
			}

			var summaries []machineClassSummary
			for _, mc := range list.Items {
				summaries = append(summaries, machineClassSummary{
					Zone:        zc.Name,
					Name:        mc.Name,
					DisplayName: mc.Spec.DisplayName,
					Category:    mc.Spec.Category,
					CPUCores:    mc.Spec.CPU.Cores,
					Memory:      mc.Spec.Memory.Quantity.String(),
					Enabled:     mc.Spec.Enabled,
					Default:     mc.Spec.Default,
				})
			}
			return summaries, nil
		})

		return formatQueryResults("machine_classes", results)
	}
}

type machineSummary struct {
	Zone         string   `json:"zone"`
	Namespace    string   `json:"namespace"`
	Name         string   `json:"name"`
	Phase        string   `json:"phase"`
	Provider     string   `json:"provider"`
	MachineClass string   `json:"machineClass"`
	CPUs         int      `json:"cpus"`
	Memory       int64    `json:"memoryBytes"`
	IPs          []string `json:"ipAddresses,omitempty"`
	Hostname     string   `json:"hostname,omitempty"`
}

type machineDetail struct {
	machineSummary
	ProviderID string             `json:"providerID,omitempty"`
	MachineID  string             `json:"machineID,omitempty"`
	OS         machineOS          `json:"os"`
	Disks      []machineDisk      `json:"disks,omitempty"`
	Interfaces []machineInterface `json:"networkInterfaces,omitempty"`
	Conditions []machineCondition `json:"conditions,omitempty"`
	BootTime   string             `json:"bootTime,omitempty"`
}

type machineOS struct {
	Family       string `json:"family,omitempty"`
	Distribution string `json:"distribution,omitempty"`
	Version      string `json:"version,omitempty"`
	Architecture string `json:"architecture,omitempty"`
}

type machineDisk struct {
	Name   string `json:"name"`
	SizeGB int64  `json:"sizeGB"`
	Type   string `json:"type,omitempty"`
}

type machineInterface struct {
	Name        string   `json:"name"`
	MacAddress  string   `json:"macAddress,omitempty"`
	IPAddresses []string `json:"ipAddresses,omitempty"`
	State       string   `json:"state,omitempty"`
}

type machineCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type machineClassSummary struct {
	Zone        string `json:"zone"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Category    string `json:"category,omitempty"`
	CPUCores    uint   `json:"cpuCores"`
	Memory      string `json:"memory"`
	Enabled     bool   `json:"enabled"`
	Default     bool   `json:"default"`
}

func toMachineSummary(zoneName string, m *v1alpha1.Machine) machineSummary {
	return machineSummary{
		Zone:         zoneName,
		Namespace:    m.Namespace,
		Name:         m.Name,
		Phase:        m.Status.Phase,
		Provider:     string(m.Spec.Provider),
		MachineClass: m.Spec.MachineClass,
		CPUs:         m.Status.CPUs,
		Memory:       m.Status.Memory,
		IPs:          m.Status.IPAddresses,
		Hostname:     m.Status.Hostname,
	}
}

func toMachineDetail(zoneName string, m *v1alpha1.Machine) machineDetail {
	detail := machineDetail{
		machineSummary: toMachineSummary(zoneName, m),
		ProviderID:     m.Status.ProviderID,
		MachineID:      m.Status.MachineID,
		OS: machineOS{
			Family:       m.Spec.OS.Family,
			Distribution: m.Spec.OS.Distribution,
			Version:      m.Spec.OS.Version,
			Architecture: m.Spec.OS.Architecture,
		},
	}

	if m.Status.BootTime != nil && !m.Status.BootTime.IsZero() {
		detail.BootTime = m.Status.BootTime.String()
	}

	for _, d := range m.Spec.Disks {
		detail.Disks = append(detail.Disks, machineDisk{
			Name:   d.Name,
			SizeGB: d.SizeGB,
			Type:   d.Type,
		})
	}

	for _, iface := range m.Status.NetworkInterfaces {
		detail.Interfaces = append(detail.Interfaces, machineInterface{
			Name:        iface.Name,
			MacAddress:  iface.MACAddress,
			IPAddresses: iface.IPAddresses,
			State:       iface.State,
		})
	}

	for _, c := range m.Status.Conditions {
		detail.Conditions = append(detail.Conditions, machineCondition{
			Type:    c.Type,
			Status:  c.Status,
			Reason:  c.Reason,
			Message: c.Message,
		})
	}

	return detail
}
