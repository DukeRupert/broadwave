package main

import (
	"context"
	"embed"
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dukerupert/broadwave/internal/config"
	"github.com/dukerupert/broadwave/internal/database"
	"github.com/dukerupert/broadwave/internal/handler"
	"github.com/dukerupert/broadwave/internal/mailer"
	"github.com/dukerupert/broadwave/internal/ratelimit"
)

//go:embed templates/*
var templateFS embed.FS

func main() {
	configPath := flag.String("config", "config.toml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Database
	db := database.MustOpen(cfg.Database.Path)
	defer db.Close()
	database.MustMigrate(db)

	// Dependencies
	queries := database.NewQueries(db)
	m := mailer.New(cfg.Postmark.ServerToken, cfg.Postmark.MessageStream)
	limiter := ratelimit.New(5, time.Hour)

	// Parse templates once at startup
	tmpl := &handler.Templates{
		SubscribeSuccess: template.Must(template.ParseFS(templateFS, "templates/subscribe_success.html")),
		SubscribeError:   template.Must(template.ParseFS(templateFS, "templates/subscribe_error.html")),
		ConfirmSuccess:   template.Must(template.ParseFS(templateFS, "templates/confirm_success.html")),
		AlreadyConfirmed: template.Must(template.ParseFS(templateFS, "templates/already_confirmed.html")),
		Error:            template.Must(template.ParseFS(templateFS, "templates/error.html")),
	}

	deps := &handler.Deps{
		Queries:         queries,
		Mailer:          m,
		Limiter:         limiter,
		Templates:       tmpl,
		BaseURL:         cfg.App.BaseURL,
		DefaultRedirect: cfg.Subscribe.DefaultRedirect,
	}

	// Rate limiter cleanup
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			limiter.Cleanup()
		}
	}()

	// Routes
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/subscribe", deps.HandleSubscribe)
	mux.HandleFunc("GET /confirm/{token}", deps.HandleConfirm)

	srv := &http.Server{
		Addr:         cfg.App.ListenAddr,
		Handler:      handler.LoggingMiddleware(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Broadwave starting on %s", cfg.App.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Shutdown error: %v", err)
	}
	log.Println("Broadwave stopped")
}
