package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type StatefulSetView struct {
	Name      string
	Replicas  string // ready/desired
	Age       string
	Images    []string
}

type StatefulSetsListPage struct {
	BasePage
	StatefulSets []StatefulSetView
}

func (s *Server) handleStatefulSetsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ss, err := s.manager.Client().AppsV1().StatefulSets(s.manager.Namespace()).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var views []StatefulSetView
	for _, item := range ss.Items {
		var images []string
		for _, c := range item.Spec.Template.Spec.Containers {
			images = append(images, c.Image)
		}
		views = append(views, StatefulSetView{
			Name:     item.Name,
			Replicas: fmt.Sprintf("%d/%d", item.Status.ReadyReplicas, *item.Spec.Replicas),
			Age:      formatAge(item.CreationTimestamp.Time),
			Images:   images,
		})
	}

	data := StatefulSetsListPage{
		BasePage:     BasePage{Namespace: s.manager.Namespace(), Title: "StatefulSets", Active: "statefulsets"},
		StatefulSets: views,
	}

	s.renderTemplate(w, "statefulsets_list.html", data)
}

type JobView struct {
	Name        string
	Completions string // succeeded/desired
	Duration    string
	Age         string
	Status      string
}

type JobsListPage struct {
	BasePage
	Jobs []JobView
}

func (s *Server) handleJobsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobs, err := s.manager.Client().BatchV1().Jobs(s.manager.Namespace()).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var views []JobView
	for _, j := range jobs.Items {
		status := "Running"
		if j.Status.Succeeded > 0 {
			status = "Completed"
		} else if j.Status.Failed > 0 {
			status = "Failed"
		}

		duration := "-"
		if j.Status.StartTime != nil {
			end := j.Status.CompletionTime
			if end == nil {
				// If not completed, use now
				now := metav1.Now()
				end = &now
			}
			d := end.Time.Sub(j.Status.StartTime.Time)
			duration = fmt.Sprintf("%s", d.Round(time.Second))
		}

		desired := int32(1)
		if j.Spec.Completions != nil {
			desired = *j.Spec.Completions
		}

		views = append(views, JobView{
			Name:        j.Name,
			Completions: fmt.Sprintf("%d/%d", j.Status.Succeeded, desired),
			Duration:    duration,
			Age:         formatAge(j.CreationTimestamp.Time),
			Status:      status,
		})
	}

	data := JobsListPage{
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "Jobs", Active: "jobs"},
		Jobs:     views,
	}

	s.renderTemplate(w, "jobs_list.html", data)
}

type CronJobView struct {
	Name             string
	Schedule         string
	Suspend          bool
	Active           int
	LastScheduleTime string
	Age              string
}

type CronJobsListPage struct {
	BasePage
	CronJobs []CronJobView
}

func (s *Server) handleCronJobsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cjs, err := s.manager.Client().BatchV1().CronJobs(s.manager.Namespace()).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var views []CronJobView
	for _, cj := range cjs.Items {
		lastSchedule := "-"
		if cj.Status.LastScheduleTime != nil {
			lastSchedule = formatAge(cj.Status.LastScheduleTime.Time) + " ago"
		}

		suspend := false
		if cj.Spec.Suspend != nil {
			suspend = *cj.Spec.Suspend
		}

		views = append(views, CronJobView{
			Name:             cj.Name,
			Schedule:         cj.Spec.Schedule,
			Suspend:          suspend,
			Active:           len(cj.Status.Active),
			LastScheduleTime: lastSchedule,
			Age:              formatAge(cj.CreationTimestamp.Time),
		})
	}

	data := CronJobsListPage{
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "CronJobs", Active: "cronjobs"},
		CronJobs: views,
	}

	s.renderTemplate(w, "cronjobs_list.html", data)
}

func (s *Server) handleStatefulSetYAML(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	ss, err := s.manager.Client().AppsV1().StatefulSets(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ss.ManagedFields = nil
	y, err := yaml.Marshal(ss)
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
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "YAML: " + name, Active: "statefulsets"},
		Name:     name,
		Kind:     "statefulsets",
		YAML:     string(y),
	}

	s.renderTemplate(w, "yaml_view.html", data)
}

func (s *Server) handleJobYAML(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	j, err := s.manager.Client().BatchV1().Jobs(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	j.ManagedFields = nil
	y, err := yaml.Marshal(j)
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
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "YAML: " + name, Active: "jobs"},
		Name:     name,
		Kind:     "jobs",
		YAML:     string(y),
	}

	s.renderTemplate(w, "yaml_view.html", data)
}

func (s *Server) handleCronJobYAML(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	cj, err := s.manager.Client().BatchV1().CronJobs(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cj.ManagedFields = nil
	y, err := yaml.Marshal(cj)
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
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "YAML: " + name, Active: "cronjobs"},
		Name:     name,
		Kind:     "cronjobs",
		YAML:     string(y),
	}

	s.renderTemplate(w, "yaml_view.html", data)
}
