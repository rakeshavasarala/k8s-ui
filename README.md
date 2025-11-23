# k8s-ui

A single-binary, namespace-scoped Kubernetes UI console written in Go.

## Features
- **Zero Dependencies**: Single static binary with embedded templates.
- **Namespace Scoped**: Operates only within its own namespace (safe for multi-tenant).
- **Local Development Friendly**:
  - **Context Switching**: Switch between Kubernetes clusters (contexts) dynamically.
  - **Namespace Switching**: Switch namespaces on the fly.
  - **Auto-Refresh**: Toggle automatic updates for real-time monitoring.
- **Premium UI**: Modern, dark-mode optimized interface.
- **Functionality**:
  - View Pods (status, logs, details)
  - Restart/Delete Pods
  - View Deployments
  - Scale Deployments
  - Edit Deployments (YAML)
  - View Events

## Requirements
- Go 1.22+
- Kubernetes Cluster

## Development

### Run Locally
1. Ensure you have `~/.kube/config` set up.
2. Run:
   ```bash
   # Will use the current context and namespace from your kubeconfig
   go run ./cmd/server
   ```
3. Open http://localhost:8080
4. Use the dropdowns in the header to switch contexts or namespaces.

### Build Docker Image
```bash
docker build -t k8s-ui:local .
```

## Deployment

1. Apply RBAC and Deployment:
   ```bash
   kubectl apply -f k8s/rbac.yaml
   kubectl apply -f k8s/deployment.yaml
   ```

2. Port-forward to access:
   ```bash
   kubectl port-forward svc/k8s-ui 8080:80
   ```
   Open http://localhost:8080

## User Guide

For detailed usage instructions, please refer to the [User Guide](USER_GUIDE.md).
