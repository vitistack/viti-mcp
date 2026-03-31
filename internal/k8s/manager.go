package k8s

import (
	"context"
	"fmt"
	"sync"

	"github.com/vitistack/common/pkg/v1alpha1"
	"github.com/vitistack/viti-mcp/internal/config"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ZoneClient holds a K8s client and metadata for a single availability zone.
type ZoneClient struct {
	Name   string
	Region string
	Client client.Client
}

// Manager manages K8s clients for multiple availability zones.
type Manager struct {
	clients map[string]*ZoneClient
}

// NewManager creates clients for all configured availability zones.
func NewManager(cfg *config.Config) (*Manager, error) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("adding vitistack scheme: %w", err)
	}

	m := &Manager{
		clients: make(map[string]*ZoneClient, len(cfg.AvailabilityZones)),
	}

	for _, az := range cfg.AvailabilityZones {
		c, err := buildClient(az, scheme)
		if err != nil {
			return nil, fmt.Errorf("availability zone %q: %w", az.Name, err)
		}
		m.clients[az.Name] = &ZoneClient{
			Name:   az.Name,
			Region: az.Region,
			Client: c,
		}
	}

	return m, nil
}

func buildClient(az config.AvailabilityZone, scheme *runtime.Scheme) (client.Client, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{
		ExplicitPath: az.ResolvedKubeconfig(),
	}

	overrides := &clientcmd.ConfigOverrides{}
	if az.Context != "" {
		overrides.CurrentContext = az.Context
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building rest config from %s: %w", az.ResolvedKubeconfig(), err)
	}

	c, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	return c, nil
}

// Get returns the client for a specific availability zone.
func (m *Manager) Get(name string) (*ZoneClient, bool) {
	zc, ok := m.clients[name]
	return zc, ok
}

// All returns all availability zone clients.
func (m *Manager) All() []*ZoneClient {
	result := make([]*ZoneClient, 0, len(m.clients))
	for _, zc := range m.clients {
		result = append(result, zc)
	}
	return result
}

// Names returns all availability zone names.
func (m *Manager) Names() []string {
	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	return names
}

// QueryResult holds results from a single availability zone query.
type QueryResult[T any] struct {
	Zone  string
	Items []T
	Err   error
}

// QueryAll runs a query function across all availability zones in parallel.
// If zone is non-empty, only that zone is queried.
func QueryAll[T any](ctx context.Context, m *Manager, zone string, fn func(ctx context.Context, zc *ZoneClient) ([]T, error)) []QueryResult[T] {
	var targets []*ZoneClient
	if zone != "" {
		if zc, ok := m.Get(zone); ok {
			targets = []*ZoneClient{zc}
		} else {
			return []QueryResult[T]{{Zone: zone, Err: fmt.Errorf("unknown availability zone: %s", zone)}}
		}
	} else {
		targets = m.All()
	}

	results := make([]QueryResult[T], len(targets))
	var wg sync.WaitGroup
	for i, zc := range targets {
		wg.Add(1)
		go func(idx int, zc *ZoneClient) {
			defer wg.Done()
			items, err := fn(ctx, zc)
			results[idx] = QueryResult[T]{
				Zone:  zc.Name,
				Items: items,
				Err:   err,
			}
		}(i, zc)
	}
	wg.Wait()
	return results
}
