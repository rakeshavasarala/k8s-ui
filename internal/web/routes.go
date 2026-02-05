package web

import (
	"net/http"
)

func (s *Server) registerRoutes() {
	// Redirect root to /pods
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/pods", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	// Pods
	s.mux.HandleFunc("/pods", s.handlePodsList)
	s.mux.HandleFunc("/pods/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		sub := path[len("/pods/"):]
		if sub == "" {
			http.Redirect(w, r, "/pods", http.StatusFound)
			return
		}

		if len(sub) > 5 && sub[len(sub)-5:] == "/logs" {
			s.handlePodLogs(w, r)
			return
		}
		if len(sub) > 14 && sub[len(sub)-14:] == "/logs/download" {
			s.handlePodLogsDownload(w, r)
			return
		}
		if len(sub) > 5 && sub[len(sub)-5:] == "/exec" {
			s.handlePodExec(w, r)
			return
		}
		if len(sub) > 8 && sub[len(sub)-8:] == "/exec/ws" {
			s.handlePodExecWS(w, r)
			return
		}
		if len(sub) > 8 && sub[len(sub)-8:] == "/restart" {
			s.handlePodRestart(w, r)
			return
		}
		if len(sub) > 7 && sub[len(sub)-7:] == "/delete" {
			s.handlePodDelete(w, r)
			return
		}
		if len(sub) > 5 && sub[len(sub)-5:] == "/yaml" {
			s.handlePodYAML(w, r)
			return
		}

		s.handlePodDetail(w, r)
	})

	// Deployments
	s.mux.HandleFunc("/deployments", s.handleDeploymentsList)
	s.mux.HandleFunc("/deployments/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		sub := path[len("/deployments/"):]

		if len(sub) > 8 && sub[len(sub)-8:] == "/restart" {
			s.handleDeploymentRestart(w, r)
			return
		}
		if len(sub) > 6 && sub[len(sub)-6:] == "/scale" {
			s.handleDeploymentScale(w, r)
			return
		}
		if len(sub) > 5 && sub[len(sub)-5:] == "/edit" {
			if r.Method == http.MethodPost {
				s.handleDeploymentEditPOST(w, r)
			} else {
				s.handleDeploymentEditGET(w, r)
			}
			return
		}
		if len(sub) > 5 && sub[len(sub)-5:] == "/yaml" {
			s.handleDeploymentYAML(w, r)
			return
		}

		http.Redirect(w, r, "/deployments", http.StatusFound)
	})

	// Events
	s.mux.HandleFunc("/events", s.handleEventsList)

	// Workloads
	s.mux.HandleFunc("/statefulsets", s.handleStatefulSetsList)
	s.mux.HandleFunc("/statefulsets/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		sub := path[len("/statefulsets/"):]

		if len(sub) > 8 && sub[len(sub)-8:] == "/restart" {
			s.handleStatefulSetRestart(w, r)
			return
		}
		if len(sub) > 6 && sub[len(sub)-6:] == "/scale" {
			s.handleStatefulSetScale(w, r)
			return
		}
		if len(sub) > 5 && sub[len(sub)-5:] == "/yaml" {
			s.handleStatefulSetYAML(w, r)
			return
		}
		http.Redirect(w, r, "/statefulsets", http.StatusFound)
	})

	s.mux.HandleFunc("/jobs", s.handleJobsList)
	s.mux.HandleFunc("/jobs/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		sub := path[len("/jobs/"):]

		if len(sub) > 7 && sub[len(sub)-7:] == "/delete" {
			s.handleJobDelete(w, r)
			return
		}
		if len(sub) > 5 && sub[len(sub)-5:] == "/yaml" {
			s.handleJobYAML(w, r)
			return
		}
		http.Redirect(w, r, "/jobs", http.StatusFound)
	})

	s.mux.HandleFunc("/cronjobs", s.handleCronJobsList)
	s.mux.HandleFunc("/cronjobs/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		sub := path[len("/cronjobs/"):]

		if len(sub) > 8 && sub[len(sub)-8:] == "/suspend" {
			s.handleCronJobSuspend(w, r)
			return
		}
		if len(sub) > 8 && sub[len(sub)-8:] == "/trigger" {
			s.handleCronJobTrigger(w, r)
			return
		}
		if len(sub) > 5 && sub[len(sub)-5:] == "/yaml" {
			s.handleCronJobYAML(w, r)
			return
		}
		http.Redirect(w, r, "/cronjobs", http.StatusFound)
	})

	// Networking
	s.mux.HandleFunc("/services", s.handleServicesList)
	s.mux.HandleFunc("/services/", func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 5 && r.URL.Path[len(r.URL.Path)-5:] == "/yaml" {
			s.handleServiceYAML(w, r)
			return
		}
		http.Redirect(w, r, "/services", http.StatusFound)
	})

	s.mux.HandleFunc("/ingresses", s.handleIngressList)
	s.mux.HandleFunc("/ingresses/", func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 5 && r.URL.Path[len(r.URL.Path)-5:] == "/yaml" {
			s.handleIngressYAML(w, r)
			return
		}
		http.Redirect(w, r, "/ingresses", http.StatusFound)
	})

	// Config
	s.mux.HandleFunc("/configmaps", s.handleConfigMapsList)
	s.mux.HandleFunc("/configmaps/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		sub := path[len("/configmaps/"):]

		if len(sub) > 5 && sub[len(sub)-5:] == "/edit" {
			if r.Method == http.MethodPost {
				s.handleConfigMapEditPOST(w, r)
			} else {
				s.handleConfigMapEditGET(w, r)
			}
			return
		}
		if len(sub) > 5 && sub[len(sub)-5:] == "/yaml" {
			s.handleConfigMapYAML(w, r)
			return
		}
		http.Redirect(w, r, "/configmaps", http.StatusFound)
	})

	s.mux.HandleFunc("/secrets", s.handleSecretsList)
	s.mux.HandleFunc("/secrets/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		sub := path[len("/secrets/"):]

		if len(sub) > 5 && sub[len(sub)-5:] == "/yaml" {
			s.handleSecretYAML(w, r)
			return
		}

		// Detail view
		if sub != "" {
			s.handleSecretDetail(w, r)
			return
		}

		http.Redirect(w, r, "/secrets", http.StatusFound)
	})

	// Storage
	s.mux.HandleFunc("/pvcs", s.handlePVCsList)
	s.mux.HandleFunc("/pvcs/", func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 5 && r.URL.Path[len(r.URL.Path)-5:] == "/yaml" {
			s.handlePVCYAML(w, r)
			return
		}
		http.Redirect(w, r, "/pvcs", http.StatusFound)
	})

	// API
	s.mux.HandleFunc("/api/switch-context", s.handleSwitchContext)
	s.mux.HandleFunc("/api/switch-namespace", s.handleSwitchNamespace)
}
