package handler

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/dukerupert/broadwave/internal/model"
	"github.com/google/uuid"
)

func (a *AdminDeps) HandleCreateList(w http.ResponseWriter, r *http.Request) {
	a.Templates.CreateList.ExecuteTemplate(w, "layout", nil)
}

func (a *AdminDeps) HandleCreateListSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	slug := strings.TrimSpace(r.FormValue("slug"))
	description := strings.TrimSpace(r.FormValue("description"))
	fromName := strings.TrimSpace(r.FormValue("from_name"))
	fromEmail := strings.TrimSpace(strings.ToLower(r.FormValue("from_email")))

	if name == "" || slug == "" || fromName == "" || fromEmail == "" {
		http.Error(w, "Name, slug, from name, and from email are required.", http.StatusBadRequest)
		return
	}

	if _, err := mail.ParseAddress(fromEmail); err != nil {
		http.Error(w, "Invalid from email address.", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check slug uniqueness
	if _, err := a.Queries.GetListBySlug(ctx, slug); err == nil {
		http.Error(w, "A list with that slug already exists.", http.StatusBadRequest)
		return
	}

	id, err := a.Queries.CreateList(ctx, slug, name, description, fromName, fromEmail)
	if err != nil {
		log.Printf("Error creating list: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/lists/%d", id), http.StatusSeeOther)
}

func (a *AdminDeps) HandleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	listID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid list ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	label := strings.TrimSpace(r.FormValue("label"))

	ctx := r.Context()

	// Verify list exists
	if _, err := a.Queries.GetListByID(ctx, listID); err != nil {
		http.Error(w, "List not found", http.StatusNotFound)
		return
	}

	fullKey, prefix, err := generateAPIKey()
	if err != nil {
		log.Printf("Error generating API key: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if _, err := a.Queries.CreateAPIKey(ctx, listID, fullKey, prefix, label); err != nil {
		log.Printf("Error creating API key: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Fetch updated keys list for re-rendering
	keys, err := a.Queries.GetAPIKeysByList(ctx, listID)
	if err != nil {
		log.Printf("Error fetching API keys: %v", err)
	}

	data := map[string]any{
		"ListID":    listID,
		"APIKeys":   keys,
		"NewKey":    fullKey,
		"NewPrefix": prefix,
	}

	a.Templates.APIKeySection.ExecuteTemplate(w, "api_key_section", data)
}

func (a *AdminDeps) HandleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	listID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid list ID", http.StatusBadRequest)
		return
	}

	keyID, err := strconv.ParseInt(r.PathValue("keyID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid key ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	if err := a.Queries.RevokeAPIKey(ctx, keyID, listID); err != nil {
		log.Printf("Error revoking API key: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Fetch updated keys list for re-rendering
	keys, err := a.Queries.GetAPIKeysByList(ctx, listID)
	if err != nil {
		log.Printf("Error fetching API keys: %v", err)
	}

	data := map[string]any{
		"ListID":  listID,
		"APIKeys": keys,
	}

	a.Templates.APIKeySection.ExecuteTemplate(w, "api_key_section", data)
}

func (a *AdminDeps) HandleListDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid list ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	list, err := a.Queries.GetListByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "List not found", http.StatusNotFound)
			return
		}
		log.Printf("Error fetching list: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	statusFilter := r.URL.Query().Get("status")
	subscribers, err := a.Queries.GetSubscribersByList(ctx, id, statusFilter)
	if err != nil {
		log.Printf("Error fetching subscribers: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	apiKeys, err := a.Queries.GetAPIKeysByList(ctx, id)
	if err != nil {
		log.Printf("Error fetching API keys: %v", err)
	}

	data := map[string]any{
		"List":         list,
		"Subscribers":  subscribers,
		"StatusFilter": statusFilter,
		"APIKeys":      apiKeys,
		"ListID":       list.ID,
	}

	if isHTMX(r) {
		a.Templates.ListSubscriberTable.Execute(w, data)
		return
	}

	a.Templates.ListDetail.ExecuteTemplate(w, "layout", data)
}

func (a *AdminDeps) HandleAddSubscriber(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid list ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	name := strings.TrimSpace(r.FormValue("name"))

	if _, err := mail.ParseAddress(email); err != nil || email == "" {
		http.Error(w, "A valid email address is required.", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Verify list exists
	_, err = a.Queries.GetListByID(ctx, id)
	if err != nil {
		http.Error(w, "List not found", http.StatusNotFound)
		return
	}

	// Check if subscriber already exists
	sub, err := a.Queries.GetSubscriberByEmail(ctx, email)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error looking up subscriber: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	switch {
	case sub == nil:
		// New subscriber — create as confirmed (admin override)
		unsubToken := uuid.NewString()
		subID, err := a.Queries.CreateSubscriberConfirmed(ctx, email, name, unsubToken)
		if err != nil {
			log.Printf("Error creating subscriber: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if err := a.Queries.AddSubscriberToList(ctx, id, subID); err != nil {
			log.Printf("Error adding subscriber to list: %v", err)
		}

	case sub.Status == model.StatusConfirmed:
		// Already confirmed — just add to this list
		if err := a.Queries.AddSubscriberToList(ctx, id, sub.ID); err != nil {
			log.Printf("Error adding subscriber to list: %v", err)
		}

	case sub.Status == model.StatusPending:
		// Pending — confirm and add
		if sub.ConfirmToken != "" {
			if err := a.Queries.ConfirmSubscriber(ctx, sub.ConfirmToken); err != nil {
				log.Printf("Error confirming subscriber: %v", err)
			}
		}
		if err := a.Queries.AddSubscriberToList(ctx, id, sub.ID); err != nil {
			log.Printf("Error adding subscriber to list: %v", err)
		}

	case sub.Status == model.StatusUnsubscribed:
		// Reactivate and add
		if err := a.Queries.ReactivateSubscriber(ctx, sub.ID); err != nil {
			log.Printf("Error reactivating subscriber: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if err := a.Queries.AddSubscriberToList(ctx, id, sub.ID); err != nil {
			log.Printf("Error adding subscriber to list: %v", err)
		}
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/lists/%d", id), http.StatusSeeOther)
}

func (a *AdminDeps) HandleRemoveSubscriber(w http.ResponseWriter, r *http.Request) {
	listID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid list ID", http.StatusBadRequest)
		return
	}

	subscriberID, err := strconv.ParseInt(r.PathValue("subscriberID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid subscriber ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	if err := a.Queries.RemoveSubscriberFromList(ctx, listID, subscriberID); err != nil {
		log.Printf("Error removing subscriber from list: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/lists/%d", listID), http.StatusSeeOther)
}

func (a *AdminDeps) HandleExportCSV(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid list ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	list, err := a.Queries.GetListByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "List not found", http.StatusNotFound)
			return
		}
		log.Printf("Error fetching list: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	subscribers, err := a.Queries.GetSubscribersByList(ctx, id, "confirmed")
	if err != nil {
		log.Printf("Error fetching subscribers: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("%s-subscribers.csv", list.Slug)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	writer := csv.NewWriter(w)
	writer.Write([]string{"email", "name", "subscribed_at"})

	for _, s := range subscribers {
		subscribedAt := ""
		if s.SubscribedAt != nil {
			subscribedAt = s.SubscribedAt.Format("2006-01-02 15:04:05")
		}
		writer.Write([]string{s.Email, s.Name, subscribedAt})
	}

	writer.Flush()
}
