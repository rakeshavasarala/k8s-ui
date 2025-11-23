# k8s-ui User Guide

Welcome to `k8s-ui`, a lightweight, single-binary web interface for managing Kubernetes resources within a specific namespace. This guide will help you navigate and utilize the features of the application.

## Overview

`k8s-ui` provides a real-time view of your Kubernetes resources. It is designed to be simple, fast, and focused on the most common operational tasks.

**Key Features:**
*   **Namespace Scoped**: Operates within a single namespace (defined at startup).
*   **Real-time**: Fetches data directly from the Kubernetes API.
*   **Single Binary**: Easy to deploy and run.

## Navigation

The top navigation bar allows you to switch between different resource categories:

*   **Pods**: View and manage running containers.
*   **Deployments**: Manage application deployments and scaling.
*   **StatefulSets**: View stateful applications.
*   **Jobs & CronJobs**: Monitor batch jobs and scheduled tasks.
*   **ConfigMaps & Secrets**: View configuration and sensitive data.
*   **PVCs**: Monitor persistent storage claims.
*   **Events**: View cluster events for troubleshooting.

## Local Development Features

When running `k8s-ui` locally (outside of a cluster), you get access to additional features for managing your environment.

### Context & Namespace Switching
Located in the top right of the header:
*   **Context Selector**: Switch between different Kubernetes clusters (contexts) defined in your `~/.kube/config`.
*   **Namespace Selector**: Switch between namespaces within the current cluster.

### Auto-Refresh
*   **Toggle**: Click the **Auto Refresh** button in the header to enable automatic page reloading every 5 seconds.
*   **Indicator**: The icon changes to an hourglass ⏳ when active.
*   **Persistence**: Your preference is saved in the browser, so it remains active across sessions.

## Features by Resource

### Pods
The **Pods** view is your main dashboard for running workloads.

*   **List View**: Shows all pods in the namespace with their status, restarts, and age.
*   **Pod Details**: Click on a pod name to see detailed information, including containers, images, and conditions.
*   **Logs**: Click the **Logs** button to stream logs from the pod's containers. You can switch between containers if a pod has multiple.
*   **Restart**: Click the **Restart** button to delete the pod, forcing the controller (Deployment/StatefulSet) to recreate it.
*   **Delete**: Click **Delete** to remove the pod.
*   **YAML**: Click **YAML** to view the raw resource definition.

### Deployments
Manage your stateless applications.

*   **Scale**: Use the input box and **Scale** button to change the number of replicas.
*   **Restart**: Click **Restart** to perform a rollout restart (updates the `kubectl.kubernetes.io/restartedAt` annotation).
*   **Edit YAML**: Click **Edit** to modify the deployment's YAML configuration directly in the browser.
*   **View YAML**: Click **YAML** to view the current configuration.

### Workloads (StatefulSets, Jobs, CronJobs)
Monitor other workload types.

*   **StatefulSets**: View replica status and images.
*   **Jobs**: See job completion status and duration.
*   **CronJobs**: Check schedule, active jobs, and last schedule time.
*   **YAML**: All workloads support a read-only **YAML** view.

### Configuration (ConfigMaps & Secrets)
Manage application configuration.

*   **ConfigMaps**: View keys and their values.
*   **Secrets**:
    *   **List View**: Shows secret types and keys.
    *   **Detail View**: Click a secret name to view its contents. **Values are automatically base64 decoded** for easier reading.
    *   **Security Note**: Be careful when viewing secrets in a shared environment.

### Storage (PVCs)
Monitor persistent storage.

*   **PVCs**: View claim status (Bound/Pending), capacity, access modes, and storage class.

### Events
The **Events** view is crucial for troubleshooting.

*   **List**: Shows recent events in the namespace, including warnings and errors.
*   **Details**: Includes the reason, object involved, and a detailed message.

## Troubleshooting

If you encounter issues:
1.  Check the **Events** tab for cluster-level errors.
2.  Check **Pod Logs** for application-level errors.
3.  Verify the application is running in the correct namespace (displayed in the top right).
