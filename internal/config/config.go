package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for the viti-mcp server.
type Config struct {
	AvailabilityZones []AvailabilityZone `yaml:"availabilityZones"`
}

// AvailabilityZone defines a connection to a management cluster in an availability zone.
// Each AZ can have multiple KubernetesProviders and MachineProviders.
type AvailabilityZone struct {
	Name       string `yaml:"name"`
	Kubeconfig string `yaml:"kubeconfig"`
	Context    string `yaml:"context,omitempty"`
	Region     string `yaml:"region,omitempty"`

	// KubeconfigDir is an alternative to Kubeconfig. When set, the kubeconfig
	// is read from a file named after the AZ inside this directory.
	// This is useful when kubeconfigs are mounted from Kubernetes secrets.
	// Example: kubeconfigDir: /etc/viti-mcp/kubeconfigs → reads /etc/viti-mcp/kubeconfigs/<name>
	KubeconfigDir string `yaml:"kubeconfigDir,omitempty"`
}

// ResolvedKubeconfig returns the effective kubeconfig path for this availability zone.
func (az *AvailabilityZone) ResolvedKubeconfig() string {
	if az.Kubeconfig != "" {
		return az.Kubeconfig
	}
	if az.KubeconfigDir != "" {
		return filepath.Join(az.KubeconfigDir, az.Name)
	}
	return ""
}

// Load reads the config file from the given path.
func Load(path string) (*Config, error) {
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath) // #nosec G304 -- path is from CLI flag, not user input
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	// Expand environment variables in the config (e.g. $HOME)
	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}

	if len(cfg.AvailabilityZones) == 0 {
		return nil, fmt.Errorf("config file %s: no availability zones defined", path)
	}

	for i, az := range cfg.AvailabilityZones {
		if az.Name == "" {
			return nil, fmt.Errorf("config file %s: availability zone at index %d has no name", path, i)
		}
		if az.ResolvedKubeconfig() == "" {
			return nil, fmt.Errorf("config file %s: availability zone %q has no kubeconfig or kubeconfigDir", path, az.Name)
		}
	}

	return &cfg, nil
}
