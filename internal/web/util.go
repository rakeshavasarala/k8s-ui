package web

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"reflect"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BasePage is embedded in all page models to provide common data.
type BasePage struct {
	Title            string
	Active           string
	Namespace        string
	Contexts         []string
	CurrentContext   string
	Namespaces       []string // Optional: if we want to list all available namespaces
	CurrentNamespace string
	IsLocal          bool
} // e.g., "pods", "deployments"

// FuncMap returns the template function map.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"formatAge":          formatAge,
		"readyContainers":    readyContainers,
		"totalRestarts":      totalRestarts,
		"getFirstContainer":  getFirstContainerName,
		"sub":                func(a, b int) int { return a - b },
		"add":                func(a, b int) int { return a + b },
	}
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func readyContainers(p corev1.Pod) string {
	total := len(p.Spec.Containers)
	ready := 0
	for _, s := range p.Status.ContainerStatuses {
		if s.Ready {
			ready++
		}
	}
	return fmt.Sprintf("%d/%d", ready, total)
}

func totalRestarts(p corev1.Pod) int32 {
	var restarts int32
	for _, s := range p.Status.ContainerStatuses {
		restarts += s.RestartCount
	}
	return restarts
}

func getFirstContainerName(p corev1.Pod) string {
	if len(p.Spec.Containers) > 0 {
		return p.Spec.Containers[0].Name
	}
	return ""
}

func (s *Server) renderTemplate(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Clone the layout template to ensure thread safety and avoid polluting the base template
	tmpl, err := s.layoutTmpl.Clone()
	if err != nil {
		http.Error(w, "Template clone error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse the specific page template
	// We assume templates are in the "templates" directory in the embed.FS
	_, err = tmpl.ParseFS(templateFS, "templates/"+name)
	if err != nil {
		http.Error(w, "Template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Inject common data if data is a pointer to a struct that embeds BasePage
	// Or if it's just a struct that has BasePage fields.
	// Since Go doesn't have easy dynamic typing for this without reflection, 
	// we'll just assume the caller has populated what they can, and we might need a better way 
	// to inject global state. 
	// A better approach for this project: Use a helper to construct BasePage.
	
	// However, to avoid changing every handler's struct initialization right now, 
	// let's use reflection or just rely on the handlers calling a helper.
	// But wait, I already updated handlers to set BasePage. 
	// I should probably update the BasePage in the data if possible, or just pass the manager info 
	// to the template via a separate funcmap or just ensure handlers call a helper.
	
	// Let's create a helper method on Server to get the base page data
	// and update handlers to use it? No, that's too much refactoring.
	
	// Let's just use a helper function `enrichBasePage` that uses reflection to set BasePage fields if present.
	// Or simpler: just pass the manager to the template and let the template call methods? 
	// No, templates shouldn't call complex logic.

	// Let's go with: Handlers are responsible for creating the Page struct. 
	// I will create a helper `s.basePage(title, active)` that returns a filled BasePage.
	// But I already updated handlers to do `BasePage{...}`. 
	// I should update `util.go` to include a helper and then (unfortunately) update handlers again 
	// OR just use reflection here to update the BasePage field.
	
	// Reflection approach for now to minimize churn, or just accept that I need to update handlers to use a helper.
	// Using a helper is cleaner. `s.newBasePage(title, active)`
	
	// Actually, I can just inject the "Contexts" and "Namespaces" into the data if it's a map, 
	// but it's a struct.
	
	// Let's stick to the plan: Update handlers to use `s.manager.Client()` was done.
	// Now I need to populate the new BasePage fields.
	// I will add a `enrichBasePage` method that uses reflection to find the BasePage field and set it.
	// I will add a `enrichBasePage` method that uses reflection to find the BasePage field and set it.
	data = s.enrichBasePage(data)

	// Execute the specific template (usually the one that defines "content")
	// Note: We execute "layout.html" because all pages start with {{template "layout.html" .}}
	// Wait, if we execute "layout.html", it renders the layout.
	// The page template defines "content" block.
	// So executing "layout.html" is correct.
	// However, the page template itself might be the entry point if it just defines blocks.
	// But our pages start with {{template "layout.html" .}}.
	// Actually, if we execute the *file* template (e.g. "pods_list.html"), it will invoke layout.
	// But ParseFS parses the file and adds it to the set. The name of the template added is the filename.
	
	err = tmpl.ExecuteTemplate(w, name, data)
	if err != nil {
		fmt.Printf("Error rendering template %s: %v\n", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (s *Server) enrichBasePage(data any) any {
	v := reflect.ValueOf(data)
	
	// If it's a pointer, we can modify in place
	if v.Kind() == reflect.Ptr {
		if v.Elem().Kind() != reflect.Struct {
			return data
		}
		// Check if the struct has a BasePage field
		f := v.Elem().FieldByName("BasePage")
		if !f.IsValid() || !f.CanSet() {
			return data
		}
		s.updateBasePageField(f)
		return data
	}

	// If it's a struct, we need to make a addressable copy
	if v.Kind() == reflect.Struct {
		vp := reflect.New(v.Type()) // pointer to new struct
		vp.Elem().Set(v) // copy value
		
		f := vp.Elem().FieldByName("BasePage")
		if f.IsValid() && f.CanSet() {
			s.updateBasePageField(f)
			return vp.Interface() // return the pointer to the new struct
		}
	}
	
	return data
}

func (s *Server) updateBasePageField(f reflect.Value) {
	// Get current state from manager
	contexts, currentContext := s.manager.Contexts()
	isLocal := s.manager.IsLocal()
	
	var namespaces []string
	if s.manager.Client() != nil {
		nsList, err := s.manager.Client().CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
		if err == nil {
			for _, ns := range nsList.Items {
				namespaces = append(namespaces, ns.Name)
			}
			sort.Strings(namespaces)
		}
	}

	currentBase := f.Interface().(BasePage)
	
	newBase := BasePage{
		Title:            currentBase.Title,
		Active:           currentBase.Active,
		Namespace:        s.manager.Namespace(),
		Contexts:         contexts,
		CurrentContext:   currentContext,
		Namespaces:       namespaces,
		CurrentNamespace: s.manager.Namespace(),
		IsLocal:          isLocal,
	}

	f.Set(reflect.ValueOf(newBase))
}
