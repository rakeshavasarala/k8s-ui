package web

import (
	"fmt"
	"net/http"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

type ResourceItem struct {
	Label    string
	Subtitle string
	URL      string
	Search   string
}

type ResourceGroup struct {
	Name  string
	Items []ResourceItem
}

type ResourcesIndexPage struct {
	BasePage
	Groups           []ResourceGroup
	DiscoveryWarning string
}

func (s *Server) handleResourcesIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	groups := baseResourceGroups()
	crdItems, warning := s.discoverCRDResourceItems(r)
	if len(crdItems) > 0 {
		groups = append(groups, ResourceGroup{Name: "Custom Resources", Items: crdItems})
	}

	data := ResourcesIndexPage{
		BasePage:         BasePage{Namespace: s.manager.Namespace(), Title: "Resources", Active: "resources"},
		Groups:           groups,
		DiscoveryWarning: warning,
	}

	s.renderTemplate(w, "resources_index.html", data)
}

func baseResourceGroups() []ResourceGroup {
	return []ResourceGroup{
		{
			Name: "Workloads",
			Items: []ResourceItem{
				{Label: "Pods", Subtitle: "core/v1", URL: "/pods", Search: "pods core v1 workloads"},
				{Label: "Deployments", Subtitle: "apps/v1", URL: "/deployments", Search: "deployments apps v1 workloads"},
				{Label: "StatefulSets", Subtitle: "apps/v1", URL: "/statefulsets", Search: "statefulsets apps v1 workloads"},
				{Label: "Jobs", Subtitle: "batch/v1", URL: "/jobs", Search: "jobs batch v1 workloads"},
				{Label: "CronJobs", Subtitle: "batch/v1", URL: "/cronjobs", Search: "cronjobs batch v1 workloads"},
			},
		},
		{
			Name: "Configuration",
			Items: []ResourceItem{
				{Label: "ConfigMaps", Subtitle: "core/v1", URL: "/configmaps", Search: "configmaps core v1 configuration"},
				{Label: "Secrets", Subtitle: "core/v1", URL: "/secrets", Search: "secrets core v1 configuration"},
			},
		},
		{
			Name: "Networking",
			Items: []ResourceItem{
				{Label: "Services", Subtitle: "core/v1", URL: "/services", Search: "services core v1 networking"},
				{Label: "Ingresses", Subtitle: "networking.k8s.io/v1", URL: "/ingresses", Search: "ingresses networking k8s io v1 networking"},
			},
		},
		{
			Name: "Storage",
			Items: []ResourceItem{
				{Label: "PersistentVolumeClaims", Subtitle: "core/v1", URL: "/pvcs", Search: "persistentvolumeclaims pvcs core v1 storage"},
			},
		},
		{
			Name: "Observability",
			Items: []ResourceItem{
				{Label: "Events", Subtitle: "core/v1", URL: "/events", Search: "events core v1 observability"},
			},
		},
	}
}

func (s *Server) discoverCRDResourceItems(r *http.Request) ([]ResourceItem, string) {
	cfg, err := s.manager.RESTConfig()
	if err != nil {
		return nil, "Unable to load Kubernetes client config for CRD discovery."
	}

	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		if apierrors.IsForbidden(err) {
			return nil, "Custom resources are hidden: RBAC does not allow API discovery."
		}
		return nil, "Unable to create discovery client for custom resources."
	}

	resourceLists, err := disco.ServerPreferredNamespacedResources()
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		if apierrors.IsForbidden(err) {
			return nil, "Custom resources are hidden: RBAC does not allow API discovery."
		}
		return nil, "Failed to discover custom resources from the API server."
	}

	items := make([]ResourceItem, 0)
	for _, rl := range resourceLists {
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil || isBuiltInAPIGroup(gv.Group) {
			continue
		}

		for _, res := range rl.APIResources {
			if !res.Namespaced || !supportsVerb(res.Verbs, "list") || !supportsVerb(res.Verbs, "get") {
				continue
			}
			if len(res.Name) == 0 || containsSlash(res.Name) {
				continue
			}

			items = append(items, ResourceItem{
				Label:    res.Name,
				Subtitle: fmt.Sprintf("%s/%s (%s)", gv.Group, gv.Version, res.Kind),
				URL:      fmt.Sprintf("/crds/%s/%s/%s", gv.Group, gv.Version, res.Name),
				Search:   fmt.Sprintf("%s %s %s %s %s", res.Name, res.Kind, gv.Group, gv.Version, "custom resource crd"),
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})

	return items, ""
}

func containsSlash(s string) bool {
	for _, c := range s {
		if c == '/' {
			return true
		}
	}
	return false
}
