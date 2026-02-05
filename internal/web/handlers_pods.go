package web

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"
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

	// Get pod to fetch container list
	pod, err := s.manager.Client().CoreV1().Pods(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build container list
	var containerNames []string
	for _, c := range pod.Spec.Containers {
		containerNames = append(containerNames, c.Name)
	}
	if len(containerNames) == 0 {
		http.Error(w, "No containers found in pod", http.StatusBadRequest)
		return
	}

	container := r.URL.Query().Get("container")
	// Default to first container if not specified
	if container == "" {
		container = containerNames[0]
	}

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
			Name       string
			Container  string
			Containers []string
			Logs       string
			TailLines  int64
			Follow     bool
		}{
			BasePage:   BasePage{Namespace: s.manager.Namespace(), Title: "Logs: " + name, Active: "pods"},
			Name:       name,
			Container:  container,
			Containers: containerNames,
			Logs:       buf.String(),
			TailLines:  tailLines,
			Follow:     false,
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

// handlePodLogsDownload downloads pod logs as a file
func (s *Server) handlePodLogsDownload(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	// Get pod to fetch container list
	pod, err := s.manager.Client().CoreV1().Pods(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get container from query or default to first
	container := r.URL.Query().Get("container")
	if container == "" && len(pod.Spec.Containers) > 0 {
		container = pod.Spec.Containers[0].Name
	}

	// Check for previous logs
	previous := r.URL.Query().Get("previous") == "true"

	opts := &corev1.PodLogOptions{
		Container: container,
		Previous:  previous,
	}

	req := s.manager.Client().CoreV1().Pods(s.manager.Namespace()).GetLogs(name, opts)
	stream, err := req.Stream(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	// Set headers for file download
	filename := fmt.Sprintf("%s-%s.log", name, container)
	if previous {
		filename = fmt.Sprintf("%s-%s-previous.log", name, container)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	// Stream logs to response
	_, err = io.Copy(w, stream)
	if err != nil {
		// Can't send error as headers already sent
		return
	}
}

// handlePodExec renders the exec terminal page
func (s *Server) handlePodExec(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	// Get pod to fetch container list
	pod, err := s.manager.Client().CoreV1().Pods(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var containerNames []string
	for _, c := range pod.Spec.Containers {
		containerNames = append(containerNames, c.Name)
	}

	container := r.URL.Query().Get("container")
	if container == "" && len(containerNames) > 0 {
		container = containerNames[0]
	}

	data := struct {
		BasePage
		Name       string
		Container  string
		Containers []string
	}{
		BasePage:   BasePage{Namespace: s.manager.Namespace(), Title: "Exec: " + name, Active: "pods"},
		Name:       name,
		Container:  container,
		Containers: containerNames,
	}

	s.renderTemplate(w, "pods_exec.html", data)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin
	},
}

// TerminalMessage represents a message to/from the terminal
type TerminalMessage struct {
	Type string `json:"type"` // "input", "output", "resize"
	Data string `json:"data,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
}

// handlePodExecWS handles the WebSocket connection for exec
func (s *Server) handlePodExecWS(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	container := r.URL.Query().Get("container")
	if container == "" {
		http.Error(w, "Container is required", http.StatusBadRequest)
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade to WebSocket: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Get REST config
	restConfig, err := s.manager.RESTConfig()
	if err != nil {
		conn.WriteJSON(TerminalMessage{Type: "output", Data: "Error getting REST config: " + err.Error()})
		return
	}

	// Create exec request
	req := s.manager.Client().CoreV1().RESTClient().Post().
		Resource("pods").
		Name(name).
		Namespace(s.manager.Namespace()).
		SubResource("exec").
		Param("container", container).
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "true").
		Param("command", "/bin/sh").
		Param("command", "-c").
		Param("command", "TERM=xterm-256color; export TERM; [ -x /bin/bash ] && exec /bin/bash || exec /bin/sh")

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		conn.WriteJSON(TerminalMessage{Type: "output", Data: "Error creating executor: " + err.Error()})
		return
	}

	// Create pipes for stdin/stdout
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	// Terminal size handler
	sizeChan := make(chan remotecommand.TerminalSize, 1)
	// Initial size
	sizeChan <- remotecommand.TerminalSize{Width: 120, Height: 30}

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Read from WebSocket and write to stdin
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stdinWriter.Close()
		for {
			select {
			case <-done:
				return
			default:
				_, message, err := conn.ReadMessage()
				if err != nil {
					return
				}

				var msg TerminalMessage
				if err := json.Unmarshal(message, &msg); err != nil {
					continue
				}

				switch msg.Type {
				case "input":
					stdinWriter.Write([]byte(msg.Data))
				case "resize":
					select {
					case sizeChan <- remotecommand.TerminalSize{Width: msg.Cols, Height: msg.Rows}:
					default:
					}
				}
			}
		}
	}()

	// Read from stdout and write to WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := stdoutReader.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				msg := TerminalMessage{Type: "output", Data: string(buf[:n])}
				if err := conn.WriteJSON(msg); err != nil {
					return
				}
			}
		}
	}()

	// Run the exec
	err = exec.StreamWithContext(r.Context(), remotecommand.StreamOptions{
		Stdin:             stdinReader,
		Stdout:            stdoutWriter,
		Stderr:            stdoutWriter,
		Tty:               true,
		TerminalSizeQueue: &terminalSizeQueue{sizeChan: sizeChan},
	})

	close(done)
	stdinReader.Close()
	stdoutWriter.Close()

	if err != nil {
		conn.WriteJSON(TerminalMessage{Type: "output", Data: "\r\n\r\nSession ended: " + err.Error()})
	} else {
		conn.WriteJSON(TerminalMessage{Type: "output", Data: "\r\n\r\nSession ended."})
	}

	wg.Wait()
}

// terminalSizeQueue implements remotecommand.TerminalSizeQueue
type terminalSizeQueue struct {
	sizeChan chan remotecommand.TerminalSize
}

func (t *terminalSizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-t.sizeChan
	if !ok {
		return nil
	}
	return &size
}
