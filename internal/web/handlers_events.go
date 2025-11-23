package web

import (
	"net/http"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EventView struct {
	Type    string
	Reason  string
	Message string
	Object  string
	Age     string
}

type EventsListPage struct {
	BasePage
	Events []EventView
}

func (s *Server) handleEventsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	events, err := s.manager.Client().CoreV1().Events(s.manager.Namespace()).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Sort by LastTimestamp descending
	sort.Slice(events.Items, func(i, j int) bool {
		return events.Items[i].LastTimestamp.Time.After(events.Items[j].LastTimestamp.Time)
	})

	var views []EventView
	for _, e := range events.Items {
		views = append(views, EventView{
			Type:    e.Type,
			Reason:  e.Reason,
			Message: e.Message,
			Object:  e.InvolvedObject.Kind + "/" + e.InvolvedObject.Name,
			Age:     formatAge(e.LastTimestamp.Time),
		})
	}

	data := EventsListPage{
		BasePage: BasePage{Namespace: s.manager.Namespace(), Title: "Events", Active: "events"},
		Events:   views,
	}

	s.renderTemplate(w, "events_list.html", data)
}
