package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

type DeploymentView struct {
	Name        string
	Ready       string
	Replicas    int32
	Available   int32
	Unavailable int32
	Images      []string
	Age         string
}

type DeploymentsListPage struct {
	BasePage
	Deployments []DeploymentView
}

func (s *Server) handleDeploymentsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deployments, err := s.manager.Client().AppsV1().Deployments(s.manager.Namespace()).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var views []DeploymentView
	for _, d := range deployments.Items {
		var images []string
		for _, c := range d.Spec.Template.Spec.Containers {
			images = append(images, c.Image)
		}
		
		views = append(views, DeploymentView{
			Name:        d.Name,
			Ready:       fmt.Sprintf("%d/%d", d.Status.AvailableReplicas, *d.Spec.Replicas),
			Replicas:    *d.Spec.Replicas,
			Available:   d.Status.AvailableReplicas,
			Unavailable: d.Status.UnavailableReplicas,
			Images:      images,
			Age:         formatAge(d.CreationTimestamp.Time),
		})
	}

	data := DeploymentsListPage{
		BasePage:    BasePage{Namespace: s.manager.Namespace(), Title: "Deployments", Active: "deployments"},
		Deployments: views,
	}

	s.renderTemplate(w, "deployments_list.html", data)
}

func (s *Server) handleDeploymentRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// /deployments/{name}/restart
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	patchData := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"kubectl.kubernetes.io/restartedAt": time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}
	
	payload, err := json.Marshal(patchData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = s.manager.Client().AppsV1().Deployments(s.manager.Namespace()).Patch(r.Context(), name, types.MergePatchType, payload, metav1.PatchOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/deployments", http.StatusSeeOther)
}

func (s *Server) handleDeploymentScale(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// /deployments/{name}/scale
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	replicasStr := r.FormValue("replicas")
	replicas, err := strconv.ParseInt(replicasStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid replicas", http.StatusBadRequest)
		return
	}
	r32 := int32(replicas)

	// We need to get the deployment first to avoid overwriting other fields if we used Update, 
	// but here we can use Patch or just Get/Update. Get/Update is safer for simple logic.
	d, err := s.manager.Client().AppsV1().Deployments(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	d.Spec.Replicas = &r32
	_, err = s.manager.Client().AppsV1().Deployments(s.manager.Namespace()).Update(r.Context(), d, metav1.UpdateOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/deployments", http.StatusSeeOther)
}

func (s *Server) handleDeploymentEditGET(w http.ResponseWriter, r *http.Request) {
	// /deployments/{name}/edit
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	d, err := s.manager.Client().AppsV1().Deployments(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Clean up managedFields for cleaner YAML
	d.ManagedFields = nil

	y, err := yaml.Marshal(d)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		BasePage
		Name string
		YAML string
	}{
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "Edit Deployment: " + name, Active: "deployments"},
		Name:     name,
		YAML:     string(y),
	}

	s.renderTemplate(w, "deployments_edit.html", data)
}

func (s *Server) handleDeploymentEditPOST(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	yamlContent := r.FormValue("yaml")
	
	var d appsv1.Deployment
	if err := yaml.Unmarshal([]byte(yamlContent), &d); err != nil {
		http.Error(w, "Invalid YAML: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Force namespace and name to match URL to prevent confusion
	d.Namespace = s.manager.Namespace()
	d.Name = name

	_, err := s.manager.Client().AppsV1().Deployments(s.manager.Namespace()).Update(r.Context(), &d, metav1.UpdateOptions{})
	if err != nil {
		http.Error(w, "Update failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/deployments", http.StatusSeeOther)
}

func (s *Server) handleDeploymentYAML(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	d, err := s.manager.Client().AppsV1().Deployments(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	d.ManagedFields = nil
	y, err := yaml.Marshal(d)
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
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "YAML: " + name, Active: "deployments"},
		Name:     name,
		Kind:     "deployments",
		YAML:     string(y),
	}

	s.renderTemplate(w, "yaml_view.html", data)
}
