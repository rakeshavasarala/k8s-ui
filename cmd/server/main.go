package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/rakeshavasarala/k8s-ui/internal/kube"
	"github.com/rakeshavasarala/k8s-ui/internal/web"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("version=%s commit=%s date=%s\n", version, commit, date)
		return
	}
	namespace := os.Getenv("POD_NAMESPACE")
	allowedNamespaces := parseNamespaces(os.Getenv("POD_NAMESPACES"))
	// If POD_NAMESPACE is not set, we pass empty string to NewManager
	// so it can try to detect namespace from kubeconfig in local mode.
	if namespace == "" {
		log.Println("POD_NAMESPACE not set, will attempt to detect from kubeconfig if local")
	}
	if len(allowedNamespaces) > 0 {
		log.Printf("POD_NAMESPACES set, restricting UI to: %s", strings.Join(allowedNamespaces, ","))
	}

	// Initialize Kubernetes Manager
	manager, err := kube.NewManager(namespace, allowedNamespaces)
	if err != nil {
		log.Fatalf("Failed to initialize kubernetes manager: %v", err)
	}

	// Initialize Web Server
	srv, err := web.NewServer(manager)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	var port string
	if port = os.Getenv("PORT"); port == "" {
		port = "3000"
	}

	log.Printf("Starting k8s-ui on :%s in namespace %s", port, manager.Namespace())
	if err := srv.ListenAndServe(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func parseNamespaces(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))

	for _, p := range parts {
		ns := strings.TrimSpace(p)
		if ns == "" {
			continue
		}
		if _, ok := seen[ns]; ok {
			continue
		}
		seen[ns] = struct{}{}
		result = append(result, ns)
	}

	return result
}
