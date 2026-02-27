package handler

import (
	"log"
	"net/http"
)

func (a *AdminDeps) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	lists, err := a.Queries.GetAllListsWithCounts(ctx)
	if err != nil {
		log.Printf("Error fetching lists: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	messages, err := a.Queries.GetRecentMessages(ctx, 10)
	if err != nil {
		log.Printf("Error fetching messages: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Lists":    lists,
		"Messages": messages,
	}

	a.Templates.Dashboard.ExecuteTemplate(w, "layout", data)
}
