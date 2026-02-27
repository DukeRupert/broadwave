package handler

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
)

type Templates struct {
	SubscribeSuccess *template.Template
	SubscribeError   *template.Template
	ConfirmSuccess   *template.Template
	AlreadyConfirmed *template.Template
	Unsubscribed     *template.Template
	Error            *template.Template
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func isJSON(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	accept := r.Header.Get("Accept")
	return strings.Contains(ct, "application/json") || strings.Contains(accept, "application/json")
}

func respondSuccess(w http.ResponseWriter, r *http.Request, tmpl *template.Template, redirectURL string, jsonData any) {
	if isHTMX(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, nil)
		return
	}

	if isJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonData)
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func respondError(w http.ResponseWriter, r *http.Request, tmpl *template.Template, status int, message string) {
	if isHTMX(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		tmpl.Execute(w, struct{ Message string }{message})
		return
	}

	if isJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{"error": message})
		return
	}

	http.Error(w, message, status)
}
