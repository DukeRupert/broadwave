# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Broadwave is a minimal, self-hosted email list management app. It handles subscribe/unsubscribe with double opt-in, list management, and sending emails to lists. Built for internal use alongside Firefly and Packstring apps.

## Tech Stack

- **Language:** Go
- **Database:** SQLite
- **Templating:** `html/template` or `templ`
- **Frontend:** HTMX + Alpine.js + Tailwind CSS (Firefly design tokens)
- **Auth:** Single admin user, bcrypt hash, session cookie
- **Config:** TOML file or environment variables for SMTP settings

## Build & Development Commands

```bash
go build -o broadwave .       # Build the binary
go run .                       # Run the server
go test ./...                  # Run all tests
go test ./path/to/pkg          # Run tests for a specific package
go test -run TestName ./...    # Run a single test by name
go vet ./...                   # Static analysis
```

## Architecture

The full feature spec is in `project-plan.md`. Key architectural points:

**Data model (SQLite):** Five tables — `lists`, `subscribers` (one row per email, shared across lists), `list_subscribers` (many-to-many join), `messages` (draft/sending/sent/failed), `send_log` (per-subscriber delivery tracking).

**Public endpoints:**
- `POST /api/subscribe` — accepts form-urlencoded or JSON; detects HTMX requests via `HX-Request` header and returns HTML fragments instead of redirecting
- `GET /confirm/{token}` — double opt-in confirmation
- `GET /unsubscribe/{token}` — global unsubscribe (removes from all lists)

**Admin pages:** Dashboard, list detail (with subscriber filtering/CSV export), compose message (with template preview), message detail (send stats/log). All behind single-user session auth.

**Email sending:** SMTP with configurable inter-message delay (default 100ms). Template variables: `{{name}}`, `{{email}}`, `{{unsubscribe_url}}`. Every email must include List-Unsubscribe headers (RFC 8058) and a CAN-SPAM footer with physical address.

**Spam protection:** Honeypot field (`website`), rate limiting (5 req/IP/hour), mandatory double opt-in.

## Key Design Decisions

- One subscriber record per email address; list membership is tracked in the join table
- Unsubscribe is global (removes from all lists), not per-list
- Double opt-in is mandatory, never optional
- No open relay — sending is admin-only, behind auth
- SMTP credentials live in config/env, never in the database
