package kube

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
)

// Manager handles Kubernetes client and context state.
type Manager struct {
	mu           sync.RWMutex
	clientset    kubernetes.Interface
	namespace    string
	rawConfig    api.Config
	clientConfig clientcmd.ClientConfig
	isLocal      bool
	allowedNamespaces []string
}

// NewManager initializes the manager.
// It tries to load in-cluster config first. If that fails, it falls back to
// ~/.kube/config (local mode).
func NewManager(initialNamespace string, allowedNamespaces []string) (*Manager, error) {
	m := &Manager{
		namespace:         strings.TrimSpace(initialNamespace),
		allowedNamespaces: normalizeNamespaces(allowedNamespaces),
	}
	if len(m.allowedNamespaces) > 0 {
		if m.namespace == "" || !m.isNamespaceAllowedLocked(m.namespace) {
			m.namespace = m.allowedNamespaces[0]
		}
	}

	// 1. Try in-cluster config
	config, err := rest.InClusterConfig()
	if err == nil {
		// In-Cluster mode
		m.isLocal = false
		// If namespace is not provided, try to read from /var/run/secrets/kubernetes.io/serviceaccount/namespace
		if m.namespace == "" {
			if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
				m.namespace = string(data)
			} else {
				m.namespace = "default"
			}
		}
		
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create in-cluster clientset: %w", err)
		}
		m.clientset = clientset
		return m, nil
	}

	// 2. Fallback to local kubeconfig
	m.isLocal = true
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	} else {
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	if kubeconfig == "" {
		return nil, fmt.Errorf("could not find in-cluster config and no kubeconfig found")
	}

	// Load raw config to get contexts
	m.clientConfig = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{},
	)
	
	rawConfig, err := m.clientConfig.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load raw kubeconfig: %w", err)
	}
	m.rawConfig = rawConfig

	// If namespace is not provided, use the one from current context
		if m.namespace == "" {
		ns, _, err := m.clientConfig.Namespace()
		if err == nil && ns != "" {
			m.namespace = ns
		} else {
			m.namespace = "default"
		}
	}

	// Create client for current context
	restConfig, err := m.clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create rest config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}
	m.clientset = clientset

	return m, nil
}

func (m *Manager) Client() kubernetes.Interface {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.clientset
}

func (m *Manager) Namespace() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.namespace
}

func (m *Manager) SetNamespace(ns string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.namespace = ns
}

func (m *Manager) AllowedNamespaces() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, len(m.allowedNamespaces))
	copy(out, m.allowedNamespaces)
	return out
}

func (m *Manager) IsNamespaceAllowed(ns string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isNamespaceAllowedLocked(ns)
}

func (m *Manager) IsLocal() bool {
	return m.isLocal
}

func (m *Manager) Contexts() ([]string, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if !m.isLocal {
		return nil, ""
	}

	var contexts []string
	for name := range m.rawConfig.Contexts {
		contexts = append(contexts, name)
	}
	sort.Strings(contexts)
	
	return contexts, m.rawConfig.CurrentContext
}

func (m *Manager) SwitchContext(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isLocal {
		return fmt.Errorf("cannot switch context in in-cluster mode")
	}

	if _, ok := m.rawConfig.Contexts[name]; !ok {
		return fmt.Errorf("context %s not found", name)
	}

	// Update current context in raw config (in memory only)
	m.rawConfig.CurrentContext = name
	
	// Re-create client config with override
	overrides := &clientcmd.ConfigOverrides{CurrentContext: name}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		overrides,
	)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to create rest config for context %s: %w", name, err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create clientset for context %s: %w", name, err)
	}

	m.clientset = clientset
	m.clientConfig = clientConfig
	
	// Update namespace to the new context's default if available, or keep current?
	// Usually switching context implies switching to that context's namespace.
	ns, _, err := clientConfig.Namespace()
	if err == nil && ns != "" {
		m.namespace = ns
	} else {
		m.namespace = "default"
	}

	return nil
}

// RESTConfig returns the REST config for the current context.
func (m *Manager) RESTConfig() (*rest.Config, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.isLocal {
		// In-cluster mode
		return rest.InClusterConfig()
	}

	return m.clientConfig.ClientConfig()
}

func (m *Manager) isNamespaceAllowedLocked(ns string) bool {
	if len(m.allowedNamespaces) == 0 {
		return true
	}
	ns = strings.TrimSpace(ns)
	if ns == "" {
		return false
	}
	for _, allowed := range m.allowedNamespaces {
		if allowed == ns {
			return true
		}
	}
	return false
}

func normalizeNamespaces(namespaces []string) []string {
	if len(namespaces) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(namespaces))
	result := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		clean := strings.TrimSpace(ns)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}
