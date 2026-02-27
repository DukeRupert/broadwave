package handler

import (
	"html/template"
	"log"
	"net/http"

	"github.com/dukerupert/broadwave/internal/database"
	"github.com/dukerupert/broadwave/internal/mailer"
	"github.com/dukerupert/broadwave/internal/session"
	"golang.org/x/crypto/bcrypt"
)

type AdminDeps struct {
	Queries         *database.Queries
	Sessions        *session.Store
	Templates       *AdminTemplates
	Mailer          *mailer.Mailer
	BaseURL         string
	PhysicalAddress string
	Username        string
	PasswordHash    string
}

type AdminTemplates struct {
	Login               *template.Template
	Dashboard           *template.Template
	CreateList          *template.Template
	ListDetail          *template.Template
	ListSubscriberTable *template.Template
	APIKeySection       *template.Template
	Compose             *template.Template
	MessageDetail       *template.Template
}

func (a *AdminDeps) HandleLogin(w http.ResponseWriter, r *http.Request) {
	a.Templates.Login.Execute(w, nil)
}

func (a *AdminDeps) HandleLoginForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		a.Templates.Login.Execute(w, map[string]string{"Error": "Invalid request."})
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username != a.Username {
		a.Templates.Login.Execute(w, map[string]string{"Error": "Invalid credentials."})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password)); err != nil {
		a.Templates.Login.Execute(w, map[string]string{"Error": "Invalid credentials."})
		return
	}

	token, err := a.Sessions.Create()
	if err != nil {
		log.Printf("Error creating session: %v", err)
		a.Templates.Login.Execute(w, map[string]string{"Error": "Something went wrong."})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "broadwave_session",
		Value:    token,
		Path:     "/admin",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

func (a *AdminDeps) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("broadwave_session"); err == nil {
		a.Sessions.Destroy(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "broadwave_session",
		Value:    "",
		Path:     "/admin",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}
