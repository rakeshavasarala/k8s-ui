package web

import (
	"net/http"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type ConfigMapView struct {
	Name string
	Keys []string
	Age  string
}

type ConfigMapsListPage struct {
	BasePage
	ConfigMaps []ConfigMapView
}

func (s *Server) handleConfigMapsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cms, err := s.manager.Client().CoreV1().ConfigMaps(s.manager.Namespace()).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var views []ConfigMapView
	for _, cm := range cms.Items {
		var keys []string
		for k := range cm.Data {
			keys = append(keys, k)
		}
		for k := range cm.BinaryData {
			keys = append(keys, k)
		}

		views = append(views, ConfigMapView{
			Name: cm.Name,
			Keys: keys,
			Age:  formatAge(cm.CreationTimestamp.Time),
		})
	}

	data := ConfigMapsListPage{
		BasePage:   BasePage{Namespace: s.manager.Namespace(), Title: "ConfigMaps", Active: "configmaps"},
		ConfigMaps: views,
	}

	s.renderTemplate(w, "configmaps_list.html", data)
}

type SecretView struct {
	Name string
	Type string
	Keys []string
	Age  string
}

type SecretsListPage struct {
	BasePage
	Secrets []SecretView
}

func (s *Server) handleSecretsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	secrets, err := s.manager.Client().CoreV1().Secrets(s.manager.Namespace()).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var views []SecretView
	for _, sec := range secrets.Items {
		var keys []string
		for k := range sec.Data {
			keys = append(keys, k)
		}
		for k := range sec.StringData {
			keys = append(keys, k)
		}

		views = append(views, SecretView{
			Name: sec.Name,
			Type: string(sec.Type),
			Keys: keys,
			Age:  formatAge(sec.CreationTimestamp.Time),
		})
	}

	data := SecretsListPage{
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "Secrets", Active: "secrets"},
		Secrets:  views,
	}

	s.renderTemplate(w, "secrets_list.html", data)
}

func (s *Server) handleConfigMapYAML(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	cm, err := s.manager.Client().CoreV1().ConfigMaps(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cm.ManagedFields = nil
	y, err := yaml.Marshal(cm)
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
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "YAML: " + name, Active: "configmaps"},
		Name:     name,
		Kind:     "configmaps",
		YAML:     string(y),
	}

	s.renderTemplate(w, "yaml_view.html", data)
}

func (s *Server) handleSecretYAML(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	sec, err := s.manager.Client().CoreV1().Secrets(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Mask data in YAML view for safety, or show it base64 encoded as is?
	// Usually "Edit YAML" shows base64. Let's keep it as is.
	sec.ManagedFields = nil
	y, err := yaml.Marshal(sec)
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
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "YAML: " + name, Active: "secrets"},
		Name:     name,
		Kind:     "secrets",
		YAML:     string(y),
	}

	s.renderTemplate(w, "yaml_view.html", data)
}

type SecretDetailView struct {
	BasePage
	Name      string
	Namespace string
	Type      string
	Age       string
	Data      map[string]string
}

func (s *Server) handleSecretDetail(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	sec, err := s.manager.Client().CoreV1().Secrets(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	decodedData := make(map[string]string)
	for k, v := range sec.Data {
		decodedData[k] = string(v)
	}
	for k, v := range sec.StringData {
		decodedData[k] = v
	}

	data := SecretDetailView{
		BasePage:  BasePage{Namespace: s.manager.Namespace(), Title: "Secret: " + name, Active: "secrets"},
		Name:      sec.Name,
		Namespace: sec.Namespace,
		Type:      string(sec.Type),
		Age:       formatAge(sec.CreationTimestamp.Time),
		Data:      decodedData,
	}

	s.renderTemplate(w, "secret_detail.html", data)
}
