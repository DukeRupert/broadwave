package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dukerupert/broadwave/internal/mailer"
	"github.com/dukerupert/broadwave/internal/model"
)

func (a *AdminDeps) HandleCompose(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	lists, err := a.Queries.GetAllListsWithCounts(ctx)
	if err != nil {
		log.Printf("Error fetching lists: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var preselectedListID int64
	if idStr := r.URL.Query().Get("list"); idStr != "" {
		preselectedListID, _ = strconv.ParseInt(idStr, 10, 64)
	}

	data := map[string]any{
		"Lists":             lists,
		"PreselectedListID": preselectedListID,
	}

	a.Templates.Compose.ExecuteTemplate(w, "layout", data)
}

func (a *AdminDeps) HandleComposePreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	subject := strings.TrimSpace(r.FormValue("subject"))
	bodyText := r.FormValue("body_text")
	bodyHTML := r.FormValue("body_html")

	renderedText := renderMessageBody(bodyText, "Test User", "test@example.com", "#")
	renderedText += buildCANSPAMFooter("Your Company", a.PhysicalAddress, "#")

	renderedHTML := ""
	if bodyHTML != "" {
		renderedHTML = renderMessageBody(bodyHTML, "Test User", "test@example.com", "#")
		renderedHTML += buildCANSPAMFooterHTML("Your Company", a.PhysicalAddress, "#")
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div class="space-y-4">`)
	fmt.Fprintf(w, `<h3 class="text-sm font-semibold text-gray-700">Subject</h3>`)
	fmt.Fprintf(w, `<p class="text-sm text-gray-900">%s</p>`, subject)

	if renderedHTML != "" {
		fmt.Fprintf(w, `<h3 class="text-sm font-semibold text-gray-700 mt-4">HTML Preview</h3>`)
		fmt.Fprintf(w, `<div class="border rounded p-4 bg-white">%s</div>`, renderedHTML)
	}

	fmt.Fprintf(w, `<h3 class="text-sm font-semibold text-gray-700 mt-4">Plain Text Preview</h3>`)
	fmt.Fprintf(w, `<pre class="text-sm text-gray-800 bg-gray-50 p-4 rounded whitespace-pre-wrap">%s</pre>`, renderedText)
	fmt.Fprintf(w, `</div>`)
}

func (a *AdminDeps) HandleSendMessage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	listIDStr := r.FormValue("list_id")
	subject := strings.TrimSpace(r.FormValue("subject"))
	bodyText := r.FormValue("body_text")
	bodyHTML := r.FormValue("body_html")

	listID, err := strconv.ParseInt(listIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid list", http.StatusBadRequest)
		return
	}

	if subject == "" {
		http.Error(w, "Subject is required", http.StatusBadRequest)
		return
	}
	if bodyText == "" {
		http.Error(w, "Plain text body is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	list, err := a.Queries.GetListByID(ctx, listID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "List not found", http.StatusNotFound)
			return
		}
		log.Printf("Error fetching list: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	subscribers, err := a.Queries.GetConfirmedSubscribersForList(ctx, listID)
	if err != nil {
		log.Printf("Error fetching subscribers: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(subscribers) == 0 {
		http.Error(w, "No confirmed subscribers on this list", http.StatusBadRequest)
		return
	}

	msgID, err := a.Queries.CreateMessage(ctx, listID, subject, bodyText, bodyHTML)
	if err != nil {
		log.Printf("Error creating message: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := a.Queries.UpdateMessageStatus(ctx, msgID, "sending"); err != nil {
		log.Printf("Error updating message status: %v", err)
	}

	msg := &model.Message{
		ID:       msgID,
		ListID:   listID,
		Subject:  subject,
		BodyText: bodyText,
		BodyHTML: bodyHTML,
		Status:   "sending",
	}

	go a.sendMessage(msg, list, subscribers)

	http.Redirect(w, r, fmt.Sprintf("/admin/messages/%d", msgID), http.StatusSeeOther)
}

func (a *AdminDeps) sendMessage(msg *model.Message, list *model.List, subscribers []model.SubscriberRow) {
	ctx := context.Background()
	from := fmt.Sprintf("%s <%s>", list.FromName, list.FromEmail)
	sentCount := 0
	failCount := 0

	for _, sub := range subscribers {
		unsubURL := fmt.Sprintf("%s/unsubscribe/%s", a.BaseURL, sub.UnsubscribeToken)

		renderedText := renderMessageBody(msg.BodyText, sub.Name, sub.Email, unsubURL)
		renderedText += buildCANSPAMFooter(list.FromName, a.PhysicalAddress, unsubURL)

		renderedHTML := ""
		if msg.BodyHTML != "" {
			renderedHTML = renderMessageBody(msg.BodyHTML, sub.Name, sub.Email, unsubURL)
			renderedHTML += buildCANSPAMFooterHTML(list.FromName, a.PhysicalAddress, unsubURL)
		}

		logID, err := a.Queries.CreateSendLogEntry(ctx, msg.ID, sub.ID)
		if err != nil {
			log.Printf("Error creating send log for subscriber %d: %v", sub.ID, err)
			failCount++
			continue
		}

		email := mailer.Email{
			From:     from,
			To:       sub.Email,
			Subject:  msg.Subject,
			TextBody: renderedText,
			HtmlBody: renderedHTML,
			Headers: []mailer.EmailHeader{
				{Name: "List-Unsubscribe", Value: fmt.Sprintf("<%s>", unsubURL)},
				{Name: "List-Unsubscribe-Post", Value: "List-Unsubscribe=One-Click"},
			},
		}

		_, sendErr := a.Mailer.Send(email)
		if sendErr != nil {
			log.Printf("Error sending to %s: %v", sub.Email, sendErr)
			if err := a.Queries.UpdateSendLogEntry(ctx, logID, "failed", sendErr.Error()); err != nil {
				log.Printf("Error updating send log: %v", err)
			}
			failCount++
		} else {
			if err := a.Queries.UpdateSendLogEntry(ctx, logID, "sent", ""); err != nil {
				log.Printf("Error updating send log: %v", err)
			}
			sentCount++
		}

		time.Sleep(100 * time.Millisecond)
	}

	if sentCount == 0 && failCount > 0 {
		if err := a.Queries.UpdateMessageStatus(ctx, msg.ID, "failed"); err != nil {
			log.Printf("Error updating message status: %v", err)
		}
	} else {
		if err := a.Queries.SetMessageSent(ctx, msg.ID, sentCount); err != nil {
			log.Printf("Error updating message status: %v", err)
		}
	}

	log.Printf("Message %d send complete: %d sent, %d failed", msg.ID, sentCount, failCount)
}

func (a *AdminDeps) HandleMessageDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid message ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	msg, err := a.Queries.GetMessageByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Message not found", http.StatusNotFound)
			return
		}
		log.Printf("Error fetching message: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	list, err := a.Queries.GetListByID(ctx, msg.ListID)
	if err != nil {
		log.Printf("Error fetching list: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	logEntries, err := a.Queries.GetSendLogByMessage(ctx, id)
	if err != nil {
		log.Printf("Error fetching send log: %v", err)
	}

	failedCount := 0
	for _, e := range logEntries {
		if e.Status == "failed" {
			failedCount++
		}
	}

	data := map[string]any{
		"Message":     msg,
		"List":        list,
		"LogEntries":  logEntries,
		"TotalCount":  len(logEntries),
		"FailedCount": failedCount,
	}

	a.Templates.MessageDetail.ExecuteTemplate(w, "layout", data)
}
