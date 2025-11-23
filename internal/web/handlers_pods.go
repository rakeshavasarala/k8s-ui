package web

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type PodView struct {
	Name     string
	Ready    string
	Status   string
	Restarts int32
	Age      string
	Node     string
}

type PodsListPage struct {
	BasePage
	Pods []PodView
}

func (s *Server) handlePodsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pods, err := s.manager.Client().CoreV1().Pods(s.manager.Namespace()).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var views []PodView
	for _, p := range pods.Items {
		views = append(views, PodView{
			Name:     p.Name,
			Ready:    readyContainers(p),
			Status:   string(p.Status.Phase),
			Restarts: totalRestarts(p),
			Age:      formatAge(p.CreationTimestamp.Time),
			Node:     p.Spec.NodeName,
		})
	}

	data := PodsListPage{
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "Pods", Active: "pods"},
		Pods:     views,
	}

	s.renderTemplate(w, "pods_list.html", data)
}

type PodContainerView struct {
	Name     string
	Image    string
	Ready    bool
	Restarts int32
}

type PodDetailPage struct {
	BasePage
	Name       string
	Status     string
	Node       string
	IP         string
	Age        string
	Labels     map[string]string
	Containers []PodContainerView
	Conditions []corev1.PodCondition
}

func (s *Server) handlePodDetail(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/pods/")
	
	pod, err := s.manager.Client().CoreV1().Pods(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var containers []PodContainerView
	for _, c := range pod.Spec.Containers {
		var restarts int32
		var ready bool
		for _, status := range pod.Status.ContainerStatuses {
			if status.Name == c.Name {
				restarts = status.RestartCount
				ready = status.Ready
				break
			}
		}
		containers = append(containers, PodContainerView{
			Name:     c.Name,
			Image:    c.Image,
			Ready:    ready,
			Restarts: restarts,
		})
	}

	data := PodDetailPage{
		BasePage:   BasePage{Namespace: s.manager.Namespace(), Title: "Pod: " + name, Active: "pods"},
		Name:       pod.Name,
		Status:     string(pod.Status.Phase),
		Node:       pod.Spec.NodeName,
		IP:         pod.Status.PodIP,
		Age:        formatAge(pod.CreationTimestamp.Time),
		Labels:     pod.Labels,
		Containers: containers,
		Conditions: pod.Status.Conditions,
	}

	s.renderTemplate(w, "pods_detail.html", data)
}

func (s *Server) handlePodRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// /pods/{name}/restart
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	err := s.manager.Client().CoreV1().Pods(s.manager.Namespace()).Delete(r.Context(), name, metav1.DeleteOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/pods", http.StatusSeeOther)
}

func (s *Server) handlePodDelete(w http.ResponseWriter, r *http.Request) {
	s.handlePodRestart(w, r) // Same logic
}

func (s *Server) handlePodLogs(w http.ResponseWriter, r *http.Request) {
	// /pods/{name}/logs
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	container := r.URL.Query().Get("container")
	tailLinesStr := r.URL.Query().Get("tailLines")
	followStr := r.URL.Query().Get("follow")

	tailLines := int64(200)
	if tailLinesStr != "" {
		if v, err := strconv.ParseInt(tailLinesStr, 10, 64); err == nil {
			tailLines = v
		}
	}

	follow := followStr == "1" || followStr == "true"

	opts := &corev1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
		Follow:    follow,
	}

	req := s.manager.Client().CoreV1().Pods(s.manager.Namespace()).GetLogs(name, opts)
	stream, err := req.Stream(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	if follow {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Transfer-Encoding", "chunked")
		
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		reader := bufio.NewReader(stream)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(w, "Error reading stream: %v\n", err)
				}
				break
			}
			fmt.Fprint(w, line)
			flusher.Flush()
		}
	} else {
		// Non-follow: read all and render template
		buf := new(strings.Builder)
		_, err := io.Copy(buf, stream)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			BasePage
			Name      string
			Container string
			Logs      string
			TailLines int64
			Follow    bool
		}{
			BasePage:  BasePage{Namespace: s.manager.Namespace(), Title: "Logs: " + name, Active: "pods"},
			Name:      name,
			Container: container,
			Logs:      buf.String(),
			TailLines: tailLines,
			Follow:    false,
		}
		s.renderTemplate(w, "pods_logs.html", data)
	}
}

func (s *Server) handlePodYAML(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	pod, err := s.manager.Client().CoreV1().Pods(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pod.ManagedFields = nil
	y, err := yaml.Marshal(pod)
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
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "YAML: " + name, Active: "pods"},
		Name:     name,
		Kind:     "pods",
		YAML:     string(y),
	}

	s.renderTemplate(w, "yaml_view.html", data)
}
