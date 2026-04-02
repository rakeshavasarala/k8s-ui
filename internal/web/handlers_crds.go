package web

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

type CRDResourceView struct {
	Group      string
	Version    string
	Resource   string
	Kind       string
	Namespaced bool
	ListURL    string
}

type CRDsListPage struct {
	BasePage
	Resources []CRDResourceView
}

type CRDItemView struct {
	Name    string
	Age     string
	YAMLURL string
}

type CRDItemsListPage struct {
	BasePage
	Group      string
	Version    string
	Resource   string
	Items      []CRDItemView
	BackURL    string
	ResourceID string
}

func (s *Server) handleCRDsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg, err := s.manager.RESTConfig()
	if err != nil {
		http.Error(w, "failed to get Kubernetes config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		if apierrors.IsForbidden(err) {
			s.renderPermissionDenied(w, "Cannot discover custom resources", "The current identity does not have permission to discover API resources.", "/resources", "resources")
			return
		}
		http.Error(w, "failed to create discovery client: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resourceLists, err := disco.ServerPreferredNamespacedResources()
	if err != nil {
		if !discovery.IsGroupDiscoveryFailedError(err) {
			if apierrors.IsForbidden(err) {
				s.renderPermissionDenied(w, "Cannot list custom resources", "The current identity is not allowed to read API discovery information for CRDs.", "/resources", "resources")
				return
			}
			http.Error(w, "failed to discover resources: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	resources := make([]CRDResourceView, 0)
	for _, rl := range resourceLists {
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			continue
		}

		if isBuiltInAPIGroup(gv.Group) {
			continue
		}

		for _, res := range rl.APIResources {
			if strings.Contains(res.Name, "/") {
				continue
			}
			if !res.Namespaced {
				continue
			}
			if !supportsVerb(res.Verbs, "list") || !supportsVerb(res.Verbs, "get") {
				continue
			}

			resources = append(resources, CRDResourceView{
				Group:      gv.Group,
				Version:    gv.Version,
				Resource:   res.Name,
				Kind:       res.Kind,
				Namespaced: res.Namespaced,
				ListURL:    fmt.Sprintf("/crds/%s/%s/%s", gv.Group, gv.Version, res.Name),
			})
		}
	}

	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Group == resources[j].Group {
			if resources[i].Resource == resources[j].Resource {
				return resources[i].Version < resources[j].Version
			}
			return resources[i].Resource < resources[j].Resource
		}
		return resources[i].Group < resources[j].Group
	})

	data := CRDsListPage{
		BasePage:  BasePage{Namespace: s.manager.Namespace(), Title: "CRDs", Active: "resources"},
		Resources: resources,
	}

	s.renderTemplate(w, "crds_list.html", data)
}

func (s *Server) handleCRDsSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/crds/")
	if path == "" {
		http.Redirect(w, r, "/crds", http.StatusFound)
		return
	}
	parts := strings.Split(path, "/")

	if len(parts) == 3 {
		s.handleCRDObjectsList(w, r, parts[0], parts[1], parts[2])
		return
	}

	if len(parts) == 5 && parts[4] == "yaml" {
		s.handleCRDYAML(w, r, parts[0], parts[1], parts[2], parts[3])
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleCRDObjectsList(w http.ResponseWriter, r *http.Request, group, version, resource string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dc, err := s.newDynamicClient()
	if err != nil {
		http.Error(w, "failed to create dynamic client: "+err.Error(), http.StatusInternalServerError)
		return
	}

	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	list, err := dc.Resource(gvr).Namespace(s.manager.Namespace()).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		if apierrors.IsForbidden(err) {
			s.renderPermissionDenied(w, "Access denied for CRD list", fmt.Sprintf("You are not allowed to list %s in namespace %s.", resource, s.manager.Namespace()), "/resources", "resources")
			return
		}
		http.Error(w, "failed to list resources: "+err.Error(), http.StatusInternalServerError)
		return
	}

	items := make([]CRDItemView, 0, len(list.Items))
	for _, it := range list.Items {
		name := it.GetName()
		items = append(items, CRDItemView{
			Name:    name,
			Age:     formatAge(it.GetCreationTimestamp().Time),
			YAMLURL: fmt.Sprintf("/crds/%s/%s/%s/%s/yaml", group, version, resource, name),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	resourceID := fmt.Sprintf("%s/%s (%s)", resource, version, group)
	data := CRDItemsListPage{
		BasePage:   BasePage{Namespace: s.manager.Namespace(), Title: "CRD Instances", Active: "resources"},
		Group:      group,
		Version:    version,
		Resource:   resource,
		Items:      items,
		BackURL:    "/resources",
		ResourceID: resourceID,
	}

	s.renderTemplate(w, "crd_items_list.html", data)
}

func (s *Server) handleCRDYAML(w http.ResponseWriter, r *http.Request, group, version, resource, name string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dc, err := s.newDynamicClient()
	if err != nil {
		http.Error(w, "failed to create dynamic client: "+err.Error(), http.StatusInternalServerError)
		return
	}

	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	obj, err := dc.Resource(gvr).Namespace(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsForbidden(err) {
			s.renderPermissionDenied(w, "Access denied for CRD YAML", fmt.Sprintf("You are not allowed to read %s/%s in namespace %s.", resource, name, s.manager.Namespace()), fmt.Sprintf("/crds/%s/%s/%s", group, version, resource), "resources")
			return
		}
		http.Error(w, "failed to get resource: "+err.Error(), http.StatusInternalServerError)
		return
	}

	obj.SetManagedFields(nil)
	y, err := yaml.Marshal(obj.Object)
	if err != nil {
		http.Error(w, "failed to marshal yaml: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		BasePage
		Name       string
		Kind       string
		YAML       string
		BackURL    string
		ResourceID string
	}{
		BasePage:   BasePage{Namespace: s.manager.Namespace(), Title: "YAML: " + name, Active: "resources"},
		Name:       name,
		Kind:       resource,
		YAML:       string(y),
		BackURL:    fmt.Sprintf("/crds/%s/%s/%s", group, version, resource),
		ResourceID: fmt.Sprintf("%s/%s (%s)", resource, version, group),
	}

	s.renderTemplate(w, "crd_yaml_view.html", data)
}

func (s *Server) newDynamicClient() (dynamic.Interface, error) {
	cfg, err := s.manager.RESTConfig()
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(cfg)
}

func supportsVerb(verbs metav1.Verbs, wanted string) bool {
	for _, v := range verbs {
		if v == wanted {
			return true
		}
	}
	return false
}

func isBuiltInAPIGroup(group string) bool {
	if group == "" {
		return true
	}

	builtIn := map[string]struct{}{
		"admissionregistration.k8s.io": {},
		"apiextensions.k8s.io":         {},
		"apiregistration.k8s.io":       {},
		"apps":                         {},
		"authentication.k8s.io":        {},
		"authorization.k8s.io":         {},
		"autoscaling":                  {},
		"batch":                        {},
		"certificates.k8s.io":          {},
		"coordination.k8s.io":          {},
		"discovery.k8s.io":             {},
		"events.k8s.io":                {},
		"flowcontrol.apiserver.k8s.io": {},
		"networking.k8s.io":            {},
		"node.k8s.io":                  {},
		"policy":                       {},
		"rbac.authorization.k8s.io":    {},
		"resource.k8s.io":              {},
		"scheduling.k8s.io":            {},
		"storage.k8s.io":               {},
	}

	_, ok := builtIn[group]
	return ok
}
