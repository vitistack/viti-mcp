package tools

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vitistack/viti-mcp/internal/k8s"
)

// Register adds all MCP tools to the server.
func Register(s *server.MCPServer, m *k8s.Manager) {
	registerOverviewTools(s, m)
	registerClusterTools(s, m)
	registerMachineTools(s, m)
	registerProviderTools(s, m)
	registerNetworkTools(s, m)
}

// formatResult marshals a value to indented JSON and returns it as a text result.
func formatResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("marshaling result: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// formatQueryResults formats results from a multi-zone query.
func formatQueryResults[T any](resourceType string, results []k8s.QueryResult[T]) (*mcp.CallToolResult, error) {
	type output struct {
		ResourceType string          `json:"resourceType"`
		Zones        []zoneResult[T] `json:"zones"`
		TotalCount   int             `json:"totalCount"`
	}

	var totalCount int
	var zoneResults []zoneResult[T]

	for _, r := range results {
		zr := zoneResult[T]{
			Zone:  r.Zone,
			Count: len(r.Items),
			Items: r.Items,
		}
		if r.Err != nil {
			errMsg := r.Err.Error()
			zr.Error = &errMsg
		}
		totalCount += len(r.Items)
		zoneResults = append(zoneResults, zr)
	}

	return formatResult(output{
		ResourceType: resourceType,
		Zones:        zoneResults,
		TotalCount:   totalCount,
	})
}

type zoneResult[T any] struct {
	Zone  string  `json:"zone"`
	Count int     `json:"count"`
	Items []T     `json:"items"`
	Error *string `json:"error,omitempty"`
}
