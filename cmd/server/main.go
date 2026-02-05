package main

import (
	"fmt"
	"log"
	"os"

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
	// If POD_NAMESPACE is not set, we pass empty string to NewManager
	// so it can try to detect namespace from kubeconfig in local mode.
	if namespace == "" {
		log.Println("POD_NAMESPACE not set, will attempt to detect from kubeconfig if local")
	}

	// Initialize Kubernetes Manager
	manager, err := kube.NewManager(namespace)
	if err != nil {
		log.Fatalf("Failed to initialize kubernetes manager: %v", err)
	}

	// Initialize Web Server
	srv, err := web.NewServer(manager)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	log.Printf("Starting k8s-ui on :8080 in namespace %s", namespace)
	if err := srv.ListenAndServe(":8080"); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
