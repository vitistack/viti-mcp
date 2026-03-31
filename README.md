# viti-mcp

MCP server that gives AI assistants read access to VitiStack infrastructure across multiple availability zones.

Connects to one or more Kubernetes management clusters and exposes VitiStack CRDs (clusters, machines, providers, networks) as [MCP](https://modelcontextprotocol.io/) tools. Each availability zone can have multiple MachineProviders and KubernetesProviders.

## What it does

- Queries KubernetesClusters, Machines, MachineClasses, MachineProviders, KubernetesProviders, NetworkNamespaces, and NetworkConfigurations
- Connects to multiple availability zones in parallel from a single instance
- Works with Claude Code, GitHub Copilot, and any MCP-compatible client
- Runs locally via stdio or deployed as an SSE server with Helm

## Quick start

```bash
make build
cp config.example.yaml ~/.config/viti-mcp/config.yaml
# edit config.yaml with your availability zone kubeconfigs
```

Add to Claude Code (`~/.claude/settings.json`):

```json
{
  "mcpServers": {
    "viti": {
      "command": "/path/to/viti-mcp/bin/viti-mcp"
    }
  }
}
```

Then ask: *"Give me an overview of the infrastructure"*

## Available tools

| Tool | Description |
|------|-------------|
| `list_zones` | Configured availability zones and connectivity status |
| `infrastructure_overview` | High-level summary across all zones |
| `list_clusters` / `get_cluster` | KubernetesCluster resources |
| `list_machines` / `get_machine` | Machine resources |
| `list_machine_classes` | VM size/flavor definitions |
| `list_machine_providers` / `get_machine_provider` | Infrastructure providers (multiple per zone) |
| `list_kubernetes_providers` | K8s platform providers (multiple per zone) |
| `list_network_namespaces` / `get_network_namespace` | Network namespaces |
| `list_network_configurations` | Per-machine network configs |

## Documentation

See [docs/getting-started.md](docs/getting-started.md) for full setup instructions including:

- Availability zone configuration and RBAC
- Debugging in VS Code
- Connecting to Claude Code and GitHub Copilot
- Deploying to Kubernetes with Helm
- Example prompts

## License

See [LICENSE](LICENSE).
