package handler

import (
	"database/sql"
	"log"
	"net/http"
)

func (d *Deps) HandleConfirm(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		d.Templates.Error.Execute(w, struct{ Message string }{"Invalid confirmation link."})
		return
	}

	ctx := r.Context()

	// Look up subscriber by token
	_, err := d.Queries.GetSubscriberByConfirmToken(ctx, token)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			d.Templates.AlreadyConfirmed.Execute(w, nil)
			return
		}
		log.Printf("Error looking up confirm token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		d.Templates.Error.Execute(w, struct{ Message string }{"Something went wrong. Please try again."})
		return
	}

	// Confirm the subscriber
	if err := d.Queries.ConfirmSubscriber(ctx, token); err != nil {
		log.Printf("Error confirming subscriber: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		d.Templates.Error.Execute(w, struct{ Message string }{"Something went wrong. Please try again."})
		return
	}

	d.Templates.ConfirmSuccess.Execute(w, nil)
}
