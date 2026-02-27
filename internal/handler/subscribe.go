package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strings"

	"github.com/dukerupert/broadwave/internal/database"
	"github.com/dukerupert/broadwave/internal/mailer"
	"github.com/dukerupert/broadwave/internal/model"
	"github.com/dukerupert/broadwave/internal/ratelimit"
	"github.com/google/uuid"
)

type Deps struct {
	Queries         *database.Queries
	Mailer          *mailer.Mailer
	Limiter         *ratelimit.Limiter
	Templates       *Templates
	BaseURL         string
	DefaultRedirect string
}

type subscribeRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	List     string `json:"list"`
	Website  string `json:"website"`  // honeypot
	Redirect string `json:"redirect"` // custom redirect URL
}

func (d *Deps) HandleSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	req, err := parseSubscribeRequest(r)
	if err != nil {
		respondError(w, r, d.Templates.SubscribeError, http.StatusBadRequest, "Invalid request")
		return
	}

	// Honeypot check — return fake success
	if req.Website != "" {
		redirectURL := d.redirectURL(req.Redirect)
		respondSuccess(w, r, d.Templates.SubscribeSuccess, redirectURL, map[string]string{
			"message": "Check your inbox for a confirmation link.",
		})
		return
	}

	// Rate limit
	ip := extractIP(r)
	if !d.Limiter.Allow(ip) {
		respondError(w, r, d.Templates.SubscribeError, http.StatusTooManyRequests, "Too many requests. Please try again later.")
		return
	}

	// Validate email
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if _, err := mail.ParseAddress(email); err != nil || email == "" {
		respondError(w, r, d.Templates.SubscribeError, http.StatusBadRequest, "A valid email address is required.")
		return
	}

	// Validate list
	listSlug := strings.TrimSpace(req.List)
	if listSlug == "" {
		respondError(w, r, d.Templates.SubscribeError, http.StatusBadRequest, "A list is required.")
		return
	}

	ctx := r.Context()

	// Look up list
	list, err := d.Queries.GetListBySlug(ctx, listSlug)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, r, d.Templates.SubscribeError, http.StatusBadRequest, "List not found.")
			return
		}
		log.Printf("Error looking up list: %v", err)
		respondError(w, r, d.Templates.SubscribeError, http.StatusInternalServerError, "Something went wrong. Please try again.")
		return
	}

	// Look up existing subscriber
	sub, err := d.Queries.GetSubscriberByEmail(ctx, email)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error looking up subscriber: %v", err)
		respondError(w, r, d.Templates.SubscribeError, http.StatusInternalServerError, "Something went wrong. Please try again.")
		return
	}

	redirectURL := d.redirectURL(req.Redirect)
	successPayload := map[string]string{"message": "Check your inbox for a confirmation link."}

	switch {
	case sub == nil:
		// New subscriber
		confirmToken := uuid.NewString()
		unsubToken := uuid.NewString()
		subID, err := d.Queries.CreateSubscriber(ctx, email, strings.TrimSpace(req.Name), confirmToken, unsubToken)
		if err != nil {
			log.Printf("Error creating subscriber: %v", err)
			respondError(w, r, d.Templates.SubscribeError, http.StatusInternalServerError, "Something went wrong. Please try again.")
			return
		}

		if err := d.Queries.AddSubscriberToList(ctx, list.ID, subID); err != nil {
			log.Printf("Error adding subscriber to list: %v", err)
		}

		confirmURL := fmt.Sprintf("%s/confirm/%s", d.BaseURL, confirmToken)
		if err := d.Mailer.SendConfirmation(email, list.FromName, list.FromEmail, list.Name, confirmURL); err != nil {
			log.Printf("Error sending confirmation email: %v", err)
		}

	case sub.Status == model.StatusConfirmed:
		// Already confirmed — just add to list
		if err := d.Queries.AddSubscriberToList(ctx, list.ID, sub.ID); err != nil {
			log.Printf("Error adding subscriber to list: %v", err)
		}
		successPayload["message"] = "You've been added to the list."

	case sub.Status == model.StatusPending:
		// Resend confirmation
		confirmToken := uuid.NewString()
		if err := d.Queries.UpdateConfirmToken(ctx, sub.ID, confirmToken); err != nil {
			log.Printf("Error updating confirm token: %v", err)
			respondError(w, r, d.Templates.SubscribeError, http.StatusInternalServerError, "Something went wrong. Please try again.")
			return
		}

		// Also ensure they're on this list
		if err := d.Queries.AddSubscriberToList(ctx, list.ID, sub.ID); err != nil {
			log.Printf("Error adding subscriber to list: %v", err)
		}

		confirmURL := fmt.Sprintf("%s/confirm/%s", d.BaseURL, confirmToken)
		if err := d.Mailer.SendConfirmation(email, list.FromName, list.FromEmail, list.Name, confirmURL); err != nil {
			log.Printf("Error sending confirmation email: %v", err)
		}

	case sub.Status == model.StatusUnsubscribed:
		respondError(w, r, d.Templates.SubscribeError, http.StatusBadRequest, "This email has been unsubscribed. Contact us to re-subscribe.")
		return
	}

	respondSuccess(w, r, d.Templates.SubscribeSuccess, redirectURL, successPayload)
}

func (d *Deps) redirectURL(custom string) string {
	if custom != "" {
		return custom
	}
	return d.DefaultRedirect
}

func parseSubscribeRequest(r *http.Request) (*subscribeRequest, error) {
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		var req subscribeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, err
		}
		return &req, nil
	}

	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	return &subscribeRequest{
		Email:    r.FormValue("email"),
		Name:     r.FormValue("name"),
		List:     r.FormValue("list"),
		Website:  r.FormValue("website"),
		Redirect: r.FormValue("redirect"),
	}, nil
}
