package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vitistack/viti-mcp/internal/config"
	"github.com/vitistack/viti-mcp/internal/k8s"
	"github.com/vitistack/viti-mcp/internal/tools"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "", "Path to config file (default: $HOME/.config/viti-mcp/config.yaml)")
	transport := flag.String("transport", "stdio", "Transport mode: stdio or sse")
	port := flag.String("port", "8080", "Port for SSE transport")
	baseURL := flag.String("base-url", "", "Base URL for SSE transport (e.g. https://viti-mcp.example.com)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("viti-mcp", version)
		os.Exit(0)
	}

	if *configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Error getting home directory: %v", err)
		}
		*configPath = home + "/.config/viti-mcp/config.yaml"
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	mgr, err := k8s.NewManager(cfg)
	if err != nil {
		log.Fatalf("Error creating K8s manager: %v", err)
	}

	s := server.NewMCPServer(
		"viti-mcp",
		version,
		server.WithToolCapabilities(false),
	)

	s.AddPrompt(mcp.NewPrompt("vitistack-context",
		mcp.WithPromptDescription("Provides context about the VitiStack infrastructure platform and available tools"),
	), func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: "VitiStack infrastructure context",
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleAssistant,
					Content: mcp.TextContent{
						Type: "text",
						Text: `You have access to the VitiStack infrastructure platform via MCP tools.

Available resource types:
- KubernetesCluster: Kubernetes clusters running on various providers (Talos, AKS)
- Machine: Virtual machines running on various providers (Proxmox, KubeVirt, Azure)
- MachineClass: VM size/flavor definitions (CPU, memory, GPU)
- MachineProvider: Infrastructure providers (Proxmox, KubeVirt, Azure)
- KubernetesProvider: Kubernetes platform providers (Talos, AKS)
- NetworkNamespace: Network isolation with VLAN and IP prefixes
- NetworkConfiguration: Per-machine network interface configuration

Each availability zone can have multiple MachineProviders and KubernetesProviders.

Start with 'list_zones' to see available availability zones, or 'infrastructure_overview' for a high-level summary.`,
					},
				},
			},
		}, nil
	})

	tools.Register(s, mgr)

	log.Printf("Starting viti-mcp server (version %s) with %d availability zone(s), transport=%s",
		version, len(cfg.AvailabilityZones), *transport)

	switch *transport {
	case "stdio":
		if err := server.ServeStdio(s); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	case "sse":
		addr := ":" + *port

		sseOpts := []server.SSEOption{}
		if *baseURL != "" {
			sseOpts = append(sseOpts, server.WithBaseURL(*baseURL))
		} else {
			sseOpts = append(sseOpts, server.WithBaseURL(fmt.Sprintf("http://localhost:%s", *port)))
		}
		sseOpts = append(sseOpts,
			server.WithSSEEndpoint("/sse"),
			server.WithMessageEndpoint("/message"),
			server.WithKeepAlive(true),
			server.WithKeepAliveInterval(30*time.Second),
		)

		sseServer := server.NewSSEServer(s, sseOpts...)

		// Graceful shutdown
		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			log.Println("Shutting down SSE server...")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := sseServer.Shutdown(ctx); err != nil {
				log.Printf("Shutdown error: %v", err)
			}
		}()

		log.Printf("SSE server listening on %s", addr)
		if err := sseServer.Start(addr); err != nil {
			log.Fatalf("SSE server error: %v", err)
		}
	default:
		log.Fatalf("Unknown transport: %s (use stdio or sse)", *transport)
	}
}
