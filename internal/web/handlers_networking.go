package web

import (
	"net/http"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type IngressRuleView struct {
	Host    string
	Paths   []string
	TLS     bool
}

type IngressView struct {
	Name    string
	Class   string
	Rules   []IngressRuleView
	Age     string
}

type IngressesListPage struct {
	BasePage
	Ingresses []IngressView
}

func (s *Server) handleIngressList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ingresses, err := s.manager.Client().NetworkingV1().Ingresses(s.manager.Namespace()).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var views []IngressView
	for _, ing := range ingresses.Items {
		// Build map of TLS hosts
		tlsHosts := make(map[string]bool)
		for _, tls := range ing.Spec.TLS {
			for _, h := range tls.Hosts {
				tlsHosts[h] = true
			}
		}

		var rules []IngressRuleView
		for _, rule := range ing.Spec.Rules {
			var paths []string
			if rule.HTTP != nil {
				for _, p := range rule.HTTP.Paths {
					backend := ""
					if p.Backend.Service != nil {
						backend = p.Backend.Service.Name
						if p.Backend.Service.Port.Number != 0 {
							backend += ":" + string(rune(p.Backend.Service.Port.Number))
						} else if p.Backend.Service.Port.Name != "" {
							backend += ":" + p.Backend.Service.Port.Name
						}
					}
					pathStr := p.Path
					if backend != "" {
						pathStr += " → " + backend
					}
					paths = append(paths, pathStr)
				}
			}

			host := rule.Host
			if host == "" {
				host = "*"
			}

			rules = append(rules, IngressRuleView{
				Host:  host,
				Paths: paths,
				TLS:   tlsHosts[rule.Host],
			})
		}

		class := "-"
		if ing.Spec.IngressClassName != nil {
			class = *ing.Spec.IngressClassName
		}

		views = append(views, IngressView{
			Name:  ing.Name,
			Class: class,
			Rules: rules,
			Age:   formatAge(ing.CreationTimestamp.Time),
		})
	}

	data := IngressesListPage{
		BasePage:  BasePage{Namespace: s.manager.Namespace(), Title: "Ingresses", Active: "ingresses"},
		Ingresses: views,
	}

	s.renderTemplate(w, "ingresses_list.html", data)
}

func (s *Server) handleIngressYAML(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	ing, err := s.manager.Client().NetworkingV1().Ingresses(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ing.ManagedFields = nil
	y, err := yaml.Marshal(ing)
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
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "YAML: " + name, Active: "ingresses"},
		Name:     name,
		Kind:     "ingresses",
		YAML:     string(y),
	}

	s.renderTemplate(w, "yaml_view.html", data)
}
