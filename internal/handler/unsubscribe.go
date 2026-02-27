package handler

import (
	"database/sql"
	"log"
	"net/http"
)

func (d *Deps) HandleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		d.Templates.Error.Execute(w, struct{ Message string }{"Invalid unsubscribe link."})
		return
	}

	ctx := r.Context()

	sub, err := d.Queries.GetSubscriberByUnsubscribeToken(ctx, token)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			d.Templates.Error.Execute(w, struct{ Message string }{"This unsubscribe link is not valid."})
			return
		}
		log.Printf("Error looking up unsubscribe token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		d.Templates.Error.Execute(w, struct{ Message string }{"Something went wrong. Please try again."})
		return
	}

	if err := d.Queries.UnsubscribeGlobal(ctx, sub.ID); err != nil {
		log.Printf("Error unsubscribing: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		d.Templates.Error.Execute(w, struct{ Message string }{"Something went wrong. Please try again."})
		return
	}

	d.Templates.Unsubscribed.Execute(w, nil)
}
