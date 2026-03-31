# Getting Started with viti-mcp

An MCP (Model Context Protocol) server that gives AI assistants (Claude Code, GitHub Copilot, etc.) read access to your VitiStack infrastructure across multiple availability zones.

## Prerequisites

- Go 1.26+
- `kubectl` access to one or more VitiStack management clusters
- Kubeconfig files for each availability zone you want to connect

## 1. Build

```bash
cd ~/dev/github/viti/viti-mcp
make build
```

This produces `bin/viti-mcp`.

You can also install it to your `GOBIN`:

```bash
make install
```

## 2. Configure availability zone connections

Create the config file:

```bash
mkdir -p ~/.config/viti-mcp
cp config.example.yaml ~/.config/viti-mcp/config.yaml
```

Edit `~/.config/viti-mcp/config.yaml` to point to your actual kubeconfigs:

```yaml
availabilityZones:
  # Each entry needs a unique name and a kubeconfig path.
  # Environment variables like $HOME are expanded automatically.

  - name: no-east-az1
    kubeconfig: $HOME/.kube/no-east-az1.conf
    region: no-east          # optional: for display purposes

  - name: no-west-az1
    kubeconfig: $HOME/.kube/no-west-az1.conf
    region: no-west

  - name: aks-prod
    kubeconfig: $HOME/.kube/aks-prod.conf
    context: my-aks-context  # optional: use a specific context from the kubeconfig
    region: norwayeast
```

**Key points:**
- `name` is how you'll refer to the zone in queries (e.g. "show me machines in no-east-az1")
- `kubeconfig` is the path to the kubeconfig file for that zone's management cluster
- `context` is optional — use it when your kubeconfig has multiple contexts and you want a specific one
- `region` is optional metadata for display
- Providers (MachineProviders, KubernetesProviders) are discovered automatically from CRDs in each zone

### Verifying connectivity

Before connecting the MCP server, verify that your kubeconfigs work:

```bash
# For each availability zone, confirm you can list VitiStack resources
kubectl --kubeconfig ~/.kube/no-east-az1.conf get kubernetesclusters -A
kubectl --kubeconfig ~/.kube/no-west-az1.conf get machines -A
```

### Required RBAC permissions

The kubeconfig user/service account needs **read-only** access to VitiStack CRDs:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: viti-mcp-reader
rules:
  - apiGroups: ["vitistack.io"]
    resources:
      - kubernetesclusters
      - machines
      - machineclasses
      - machineproviders
      - kubernetesproviders
      - networknamespaces
      - networkconfigurations
      - vitistacks
    verbs: ["get", "list", "watch"]
```

## 3. Running and testing

There are three ways to run viti-mcp locally. Each serves a different purpose.

**Config file resolution:** When no `--config` flag is passed, viti-mcp looks for `~/.config/viti-mcp/config.yaml` by default. All three options below use this default path unless you override it.

### Option A: Build and run with Claude Code (recommended for daily use)

Best for: using the MCP tools normally through an AI assistant.

1. Build the binary:
   ```bash
   make build
   ```

2. Add to Claude Code (`~/.claude/settings.json`):
   ```json
   {
     "mcpServers": {
       "viti": {
         "command": "/Users/yourname/dev/github/viti/viti-mcp/bin/viti-mcp"
       }
     }
   }
   ```

   This uses the default config at `~/.config/viti-mcp/config.yaml`. To use a different config file, add the `--config` flag:

   ```json
   {
     "mcpServers": {
       "viti": {
         "command": "/Users/yourname/dev/github/viti/viti-mcp/bin/viti-mcp",
         "args": ["--config", "/path/to/other-config.yaml"]
       }
     }
   }
   ```

   For project-level config, create `.mcp.json` in the project root instead of editing `settings.json`.

3. Restart Claude Code. The viti tools appear automatically — verify with `/mcp`.

4. Prompt: *"Give me an overview of the infrastructure"*

For **GitHub Copilot in VS Code**, add to your VS Code `settings.json` (Cmd+Shift+P > "Preferences: Open User Settings (JSON)"):

```json
{
  "github.copilot.chat.mcp.servers": {
    "viti": {
      "command": "/Users/yourname/dev/github/viti/viti-mcp/bin/viti-mcp"
    }
  }
}
```

Restart VS Code, then open Copilot Chat. The MCP tools will be available in agent mode.

### Option B: Run with `go run` (no build step)

Best for: developing viti-mcp — picks up code changes on each Claude Code restart without needing `make build`.

1. Add to Claude Code (`~/.claude/settings.json`):
   ```json
   {
     "mcpServers": {
       "viti": {
         "command": "go",
         "args": ["run", "./cmd/viti-mcp"],
         "cwd": "/Users/yourname/dev/github/viti/viti-mcp"
       }
     }
   }
   ```

   This also uses the default config at `~/.config/viti-mcp/config.yaml`. To override, add `"--config", "/path/to/config.yaml"` to the args list.

2. Restart Claude Code and prompt.

Claude Code spawns `go run` each time it starts, so code changes take effect after a restart without a separate build step. Slightly slower to start than a prebuilt binary.

### Option C: Debug in VS Code with Delve

Best for: stepping through code with breakpoints to understand or troubleshoot tool handlers.

**Important:** You cannot debug and use the MCP server from Claude Code/Copilot at the same time. The debugger and the MCP client would both compete for stdin/stdout. Use this option for code inspection only.

VS Code launch configurations are included in both `viti-mcp/.vscode/launch.json` and the parent `viti/.vscode/launch.json`:

- **Debug viti-mcp** — uses `./config.yaml` in the repo root (or `viti-mcp/config.yaml` from the parent workspace). Create this file for local debugging — it is gitignored.
- **Debug viti-mcp (custom config)** — prompts you for a config path at launch

To debug:

1. Create a `config.yaml` in the viti-mcp repo root with your availability zone kubeconfigs (same format as `config.example.yaml`)
2. Set breakpoints in any tool handler (e.g. `internal/tools/clusters.go`)
3. Press **F5** or select a launch config from the Run and Debug panel
4. The server starts in the integrated terminal and waits for MCP input on stdin

The debug session uses `"console": "integratedTerminal"` to keep Delve's debug protocol separate from the MCP stdio transport.

To invoke tools during a debug session, you can use the MCP inspector:

```bash
npx @anthropic-ai/mcp-cli --server "./bin/viti-mcp --config ./config.yaml"
```

### Smoke test (optional)

Verify the server starts and connects to your availability zones without configuring a client:

```bash
# Uses default config at ~/.config/viti-mcp/config.yaml
./bin/viti-mcp

# Or specify a config file
./bin/viti-mcp --config ./config.yaml
```

You'll see `Starting viti-mcp server (version dev) with N zone(s)`. The server will appear to "hang" since it's waiting for MCP input on stdin — that's normal. `Ctrl+C` to stop.

## 4. Available tools

| Tool | Description |
|------|-------------|
| `list_zones` | List all configured availability zones and their connectivity status |
| `infrastructure_overview` | High-level summary across all zones (cluster/machine counts, health) |
| `list_clusters` | List KubernetesCluster resources (filterable by zone, namespace, phase) |
| `get_cluster` | Detailed cluster info: topology, endpoints, worker pools, status |
| `list_machines` | List Machine resources (filterable by zone, namespace, phase, provider) |
| `get_machine` | Detailed machine info: hardware specs, network interfaces, disks, conditions |
| `list_machine_classes` | List available VM flavors (CPU, memory, GPU specs) |
| `list_machine_providers` | List infrastructure providers (Proxmox, KubeVirt, Azure) with health/quota |
| `get_machine_provider` | Detailed provider info: capabilities, quota usage, health status |
| `list_kubernetes_providers` | List K8s platform providers (Talos, AKS) with version and node counts |
| `list_network_namespaces` | List network namespaces with VLAN IDs and IP prefixes |
| `get_network_namespace` | Detailed network namespace: egress IPs, associated clusters |
| `list_network_configurations` | List per-machine network configurations |

All `list_*` tools accept an optional `zone` parameter. Omit it to query all zones in parallel.

## 5. How to prompt

The AI assistant discovers the tools automatically. Just ask questions in natural language.

### Getting started

```
"What availability zones are available?"
"Give me an overview of the infrastructure"
```

### Clusters

```
"List all Kubernetes clusters"
"Show me clusters in no-east-az1"
"Which clusters are in a Failed state?"
"Show me details of the cluster 'prod-cluster-1' in namespace 'platform' on no-east-az1"
"Compare cluster versions between no-east-az1 and no-west-az1"
```

### Machines

```
"List all running machines"
"Show me failed machines across all zones"
"How many machines are running on Proxmox?"
"Get details of machine 'worker-03' in namespace 'prod' on no-west-az1"
"Which machines have the most CPU cores?"
```

### Providers and capacity

```
"What machine providers are available?"
"Show me the quota usage for the Proxmox provider in no-east-az1"
"Which provider has the most available capacity?"
"Are there any unhealthy providers?"
"What Kubernetes providers are configured?"
```

### Network

```
"List all network namespaces"
"What VLAN IDs are in use in no-west-az1?"
"Show me the network namespace for the prod cluster"
"How many network configurations exist across all zones?"
```

### Cross-zone analysis

```
"Compare infrastructure across all zones"
"Which zone has the most machines?"
"Are there any failed resources anywhere?"
"Give me a summary of all clusters and their health"
"List all machines that are not in Running state"
```

### Troubleshooting

```
"Are all zones connected and healthy?"
"Show me machines with conditions that are not True"
"Which clusters have been recently updated?"
"Are there any clusters still provisioning?"
```

### Tips for effective prompting

- **Start broad, then narrow down.** Begin with `infrastructure_overview` or `list_clusters`, then ask for details on specific resources.
- **Use zone names.** Saying "in no-east-az1" or "on aks-prod" filters results to a specific zone.
- **Combine questions.** "Show me all failed machines in no-east-az1 and tell me what's wrong with them" will trigger multiple tool calls.
- **Ask for comparisons.** The AI can query multiple zones and summarize differences.
- **Request specific formats.** "Show me a table of all clusters with their version and phase" will format the output nicely.

## Deploying to Kubernetes

For shared/team access, deploy viti-mcp as an SSE server in a management cluster using the included Helm chart.

### Docker image

Build and push the image:

```bash
make docker-build IMG=ghcr.io/vitistack/viti-mcp:0.1.0
docker push ghcr.io/vitistack/viti-mcp:0.1.0
```

### Helm install

#### Option A: Inline kubeconfigs

Provide kubeconfig content directly in values (stored as Kubernetes Secrets):

```yaml
# values-prod.yaml
availabilityZones:
  - name: no-east-az1
    region: no-east
    kubeconfig: |
      apiVersion: v1
      kind: Config
      clusters:
        - cluster:
            server: https://k8s-api.no-east-az1.example.com:6443
            certificate-authority-data: LS0t...
          name: no-east-az1
      contexts:
        - context:
            cluster: no-east-az1
            user: viti-mcp
          name: no-east-az1
      current-context: no-east-az1
      users:
        - name: viti-mcp
          user:
            token: eyJhb...

  - name: no-west-az1
    region: no-west
    kubeconfig: |
      ...
```

```bash
helm install viti-mcp ./charts/viti-mcp \
  -n viti-system --create-namespace \
  -f values-prod.yaml
```

#### Option B: Existing secrets

If you manage kubeconfig secrets separately (e.g. via External Secrets, Sealed Secrets):

```yaml
# values-prod.yaml
availabilityZones:
  - name: no-east-az1
    region: no-east
    existingSecret: no-east-az1-kubeconfig
    existingSecretKey: kubeconfig  # key in the secret

  - name: no-west-az1
    region: no-west
    existingSecret: no-west-az1-kubeconfig
```

```bash
helm install viti-mcp ./charts/viti-mcp \
  -n viti-system --create-namespace \
  -f values-prod.yaml
```

### Exposing via Ingress

```yaml
# values-prod.yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: viti-mcp.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: viti-mcp-tls
      hosts:
        - viti-mcp.example.com

baseURL: https://viti-mcp.example.com
```

### Remote cluster RBAC

The Helm chart creates RBAC for the **local** cluster. For each remote availability zone, apply a read-only ClusterRole and bind it to the service account used in the kubeconfig:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: viti-mcp-reader
rules:
  - apiGroups: ["vitistack.io"]
    resources:
      - kubernetesclusters
      - machines
      - machineclasses
      - machineproviders
      - kubernetesproviders
      - networknamespaces
      - networkconfigurations
      - vitistacks
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: viti-mcp-reader
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: viti-mcp-reader
subjects:
  - kind: ServiceAccount
    name: viti-mcp
    namespace: viti-system
```

### Connecting clients to the deployed SSE server

Once deployed, AI clients connect to the SSE endpoint instead of spawning a local binary.

**Claude Code** (`~/.claude/settings.json`):
```json
{
  "mcpServers": {
    "viti": {
      "url": "https://viti-mcp.example.com/sse"
    }
  }
}
```

**VS Code / Copilot** (`settings.json`):
```json
{
  "github.copilot.chat.mcp.servers": {
    "viti": {
      "url": "https://viti-mcp.example.com/sse"
    }
  }
}
```

## Troubleshooting

### "Error loading config"

- Check that `~/.config/viti-mcp/config.yaml` exists and is valid YAML
- Verify environment variables in paths are correct

### "zone X: building rest config"

- The kubeconfig path for that zone doesn't exist or isn't readable
- The specified context doesn't exist in the kubeconfig

### "zone X: creating client"

- The cluster API server is unreachable from your machine
- Check VPN connectivity if the cluster is behind a private network
- Verify the kubeconfig credentials haven't expired

### Tools return errors for one zone

- This is normal if one zone is temporarily unreachable
- Results from reachable zones are still returned
- The error message is included in the response for the failing zone
