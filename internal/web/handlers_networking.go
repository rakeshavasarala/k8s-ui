package web

import (
	"fmt"
	"net/http"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type ServicePortView struct {
	Name       string
	Port       int32
	TargetPort string
	Protocol   string
}

type ServiceView struct {
	Name        string
	Type        string
	ClusterIP   string
	ExternalIP  string
	Ports       []ServicePortView
	Age         string
}

type ServicesListPage struct {
	BasePage
	Services []ServiceView
}

func (s *Server) handleServicesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	services, err := s.manager.Client().CoreV1().Services(s.manager.Namespace()).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var views []ServiceView
	for _, svc := range services.Items {
		var ports []ServicePortView
		for _, p := range svc.Spec.Ports {
			ports = append(ports, ServicePortView{
				Name:       p.Name,
				Port:       p.Port,
				TargetPort: p.TargetPort.String(),
				Protocol:   string(p.Protocol),
			})
		}

		externalIP := "-"
		if svc.Spec.Type == "LoadBalancer" {
			if len(svc.Status.LoadBalancer.Ingress) > 0 {
				ing := svc.Status.LoadBalancer.Ingress[0]
				if ing.IP != "" {
					externalIP = ing.IP
				} else if ing.Hostname != "" {
					externalIP = ing.Hostname
				}
			} else {
				externalIP = "<pending>"
			}
		} else if svc.Spec.Type == "NodePort" {
			var nodePorts []string
			for _, p := range svc.Spec.Ports {
				if p.NodePort != 0 {
					nodePorts = append(nodePorts, fmt.Sprintf("%d", p.NodePort))
				}
			}
			if len(nodePorts) > 0 {
				externalIP = "NodePort: " + strings.Join(nodePorts, ", ")
			}
		} else if len(svc.Spec.ExternalIPs) > 0 {
			externalIP = strings.Join(svc.Spec.ExternalIPs, ", ")
		}

		clusterIP := svc.Spec.ClusterIP
		if clusterIP == "" || clusterIP == "None" {
			clusterIP = "None"
		}

		views = append(views, ServiceView{
			Name:       svc.Name,
			Type:       string(svc.Spec.Type),
			ClusterIP:  clusterIP,
			ExternalIP: externalIP,
			Ports:      ports,
			Age:        formatAge(svc.CreationTimestamp.Time),
		})
	}

	data := ServicesListPage{
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "Services", Active: "services"},
		Services: views,
	}

	s.renderTemplate(w, "services_list.html", data)
}

func (s *Server) handleServiceYAML(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	svc, err := s.manager.Client().CoreV1().Services(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	svc.ManagedFields = nil
	y, err := yaml.Marshal(svc)
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
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "YAML: " + name, Active: "services"},
		Name:     name,
		Kind:     "services",
		YAML:     string(y),
	}

	s.renderTemplate(w, "yaml_view.html", data)
}

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
