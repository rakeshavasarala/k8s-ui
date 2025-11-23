package web

import (
	"net/http"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type PVCView struct {
	Name        string
	Status      string
	Volume      string
	Capacity    string
	AccessModes []string
	StorageClass string
	Age         string
}

type PVCsListPage struct {
	BasePage
	PVCs []PVCView
}

func (s *Server) handlePVCsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pvcs, err := s.manager.Client().CoreV1().PersistentVolumeClaims(s.manager.Namespace()).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var views []PVCView
	for _, pvc := range pvcs.Items {
		capacity := "-"
		if q, ok := pvc.Status.Capacity["storage"]; ok {
			capacity = q.String()
		}

		var modes []string
		for _, m := range pvc.Spec.AccessModes {
			modes = append(modes, string(m))
		}

		sc := ""
		if pvc.Spec.StorageClassName != nil {
			sc = *pvc.Spec.StorageClassName
		}

		views = append(views, PVCView{
			Name:         pvc.Name,
			Status:       string(pvc.Status.Phase),
			Volume:       pvc.Spec.VolumeName,
			Capacity:     capacity,
			AccessModes:  modes,
			StorageClass: sc,
			Age:          formatAge(pvc.CreationTimestamp.Time),
		})
	}

	data := PVCsListPage{
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "PVCs", Active: "pvcs"},
		PVCs:     views,
	}

	s.renderTemplate(w, "pvcs_list.html", data)
}

func (s *Server) handlePVCYAML(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	pvc, err := s.manager.Client().CoreV1().PersistentVolumeClaims(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pvc.ManagedFields = nil
	y, err := yaml.Marshal(pvc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		BasePage
		Name string
		Kind string
		YAML string
	}{
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "YAML: " + name, Active: "pvcs"},
		Name:     name,
		Kind:     "pvcs",
		YAML:     string(y),
	}

	s.renderTemplate(w, "yaml_view.html", data)
}
