package web

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/rakeshavasarala/k8s-ui/internal/kube"
)

//go:embed templates/*.html
var templateFS embed.FS

type Server struct {
	manager    *kube.Manager
	mux        *http.ServeMux
	layoutTmpl *template.Template
}

func NewServer(m *kube.Manager) (*Server, error) {
	// Parse only the layout template initially
	tmpl, err := template.New("layout.html").Funcs(FuncMap()).ParseFS(templateFS, "templates/layout.html")
	if err != nil {
		return nil, err
	}

	s := &Server{
		manager:    m,
		mux:        http.NewServeMux(),
		layoutTmpl: tmpl,
	}

	s.registerRoutes()

	return s, nil
}

func (s *Server) handleSwitchContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.FormValue("context")
	if ctx == "" {
		http.Error(w, "Context is required", http.StatusBadRequest)
		return
	}

	err := s.manager.SwitchContext(ctx)
	if err != nil {
		http.Error(w, "Failed to switch context: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect back to referer or root
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleSwitchNamespace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ns := r.FormValue("namespace")
	if ns == "" {
		http.Error(w, "Namespace is required", http.StatusBadRequest)
		return
	}

	s.manager.SetNamespace(ns)

	// Redirect back to referer or root
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}
