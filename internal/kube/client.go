package kube

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// NewInClusterClient returns a new Kubernetes clientset.
// It tries to load in-cluster config first. If that fails, it falls back to
// ~/.kube/config (useful for local development).
func NewInClusterClient() (*kubernetes.Clientset, error) {
	// 1. Try in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		// 2. Fallback to local kubeconfig
		var kubeconfig string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		} else {
			kubeconfig = os.Getenv("KUBECONFIG")
		}

		if kubeconfig == "" {
			return nil, fmt.Errorf("could not find in-cluster config and no kubeconfig found: %w", err)
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return clientset, nil
}
