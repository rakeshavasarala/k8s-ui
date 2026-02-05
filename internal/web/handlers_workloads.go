package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

type StatefulSetView struct {
	Name         string
	Replicas     string // ready/desired
	ReplicaCount int32  // for scale form
	Age          string
	Images       []string
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
			Name:         item.Name,
			Replicas:     fmt.Sprintf("%d/%d", item.Status.ReadyReplicas, *item.Spec.Replicas),
			ReplicaCount: *item.Spec.Replicas,
			Age:          formatAge(item.CreationTimestamp.Time),
			Images:       images,
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

// StatefulSet Scale
func (s *Server) handleStatefulSetScale(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	ss, err := s.manager.Client().AppsV1().StatefulSets(s.manager.Namespace()).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ss.Spec.Replicas = &r32
	_, err = s.manager.Client().AppsV1().StatefulSets(s.manager.Namespace()).Update(r.Context(), ss, metav1.UpdateOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/statefulsets", http.StatusSeeOther)
}

// StatefulSet Restart
func (s *Server) handleStatefulSetRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	_, err = s.manager.Client().AppsV1().StatefulSets(s.manager.Namespace()).Patch(r.Context(), name, types.MergePatchType, payload, metav1.PatchOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/statefulsets", http.StatusSeeOther)
}

// CronJob Suspend/Resume
func (s *Server) handleCronJobSuspend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	// Toggle suspend state
	suspend := true
	if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
		suspend = false
	}
	cj.Spec.Suspend = &suspend

	_, err = s.manager.Client().BatchV1().CronJobs(s.manager.Namespace()).Update(r.Context(), cj, metav1.UpdateOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cronjobs", http.StatusSeeOther)
}

// CronJob Trigger (create a Job from CronJob)
func (s *Server) handleCronJobTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	// Create a Job from the CronJob spec
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-manual-%d", name, time.Now().Unix()),
			Namespace: s.manager.Namespace(),
			Labels: map[string]string{
				"job-name":   name,
				"created-by": "k8s-ui",
			},
			Annotations: map[string]string{
				"cronjob.kubernetes.io/instantiate": "manual",
			},
		},
		Spec: cj.Spec.JobTemplate.Spec,
	}

	_, err = s.manager.Client().BatchV1().Jobs(s.manager.Namespace()).Create(r.Context(), job, metav1.CreateOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/jobs", http.StatusSeeOther)
}

// Job Delete
func (s *Server) handleJobDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	name := parts[2]

	// Use propagation policy to delete associated pods
	propagationPolicy := metav1.DeletePropagationBackground
	err := s.manager.Client().BatchV1().Jobs(s.manager.Namespace()).Delete(r.Context(), name, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/jobs", http.StatusSeeOther)
}
