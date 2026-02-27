# Broadwave — Self-Hosted Email List Manager
## Feature Specification v1.0

---

## Overview

A minimal, self-hosted email list management app built in Go + SQLite. Handles subscribe, unsubscribe, double opt-in, and sending to lists. Designed to run as a lightweight service alongside Firefly and Packstring apps.

This is not Mailchimp. It manages lists and sends emails. No drag-and-drop editor, no analytics dashboard, no A/B testing. Just the plumbing — reliable, simple, and under your control.

---

## Core Requirements

1. **Lists** — Create and manage named email lists (e.g., "packstring-launch", "firefly-updates", "outfitter-seasonal")
2. **Subscribers** — Add, confirm (double opt-in), and remove subscribers from lists
3. **Sending** — Compose and send a plain text or simple HTML email to all confirmed subscribers on a list
4. **Unsubscribe** — One-click unsubscribe link in every sent email (CAN-SPAM compliance)
5. **Admin UI** — Simple web interface to manage lists, view subscribers, compose and send emails
6. **Embeddable form endpoint** — A POST endpoint that any site (Firefly, Packstring, client sites) can submit to

---

## 1 — Data Model

### SQLite Schema

```sql
CREATE TABLE lists (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    slug        TEXT NOT NULL UNIQUE,          -- "packstring-launch"
    name        TEXT NOT NULL,                 -- "Packstring Launch Notifications"
    description TEXT,                          -- internal note
    from_name   TEXT NOT NULL,                 -- "Firefly Software"
    from_email  TEXT NOT NULL,                 -- "hello@fireflysoftware.dev"
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE subscribers (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    email           TEXT NOT NULL,
    name            TEXT,                      -- optional, from signup form
    status          TEXT NOT NULL DEFAULT 'pending',  -- pending | confirmed | unsubscribed
    confirm_token   TEXT,                      -- UUID for double opt-in link
    unsubscribe_token TEXT NOT NULL,           -- UUID for one-click unsubscribe
    confirmed_at    TEXT,
    unsubscribed_at TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX idx_subscribers_email ON subscribers(email);

CREATE TABLE list_subscribers (
    list_id       INTEGER NOT NULL REFERENCES lists(id),
    subscriber_id INTEGER NOT NULL REFERENCES subscribers(id),
    subscribed_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (list_id, subscriber_id)
);

CREATE TABLE messages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    list_id     INTEGER NOT NULL REFERENCES lists(id),
    subject     TEXT NOT NULL,
    body_text   TEXT NOT NULL,                 -- plain text version
    body_html   TEXT,                          -- optional HTML version
    status      TEXT NOT NULL DEFAULT 'draft', -- draft | sending | sent | failed
    sent_at     TEXT,
    sent_count  INTEGER DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE send_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id  INTEGER NOT NULL REFERENCES messages(id),
    subscriber_id INTEGER NOT NULL REFERENCES subscribers(id),
    status      TEXT NOT NULL DEFAULT 'queued', -- queued | sent | failed | bounced
    sent_at     TEXT,
    error       TEXT
);
```

### Key Design Decisions

**One subscriber record per email, multiple list memberships.** A person who signs up for both the Packstring launch list and the Firefly updates list has one row in `subscribers` and two rows in `list_subscribers`. Unsubscribing from one list doesn't affect the other.

**Global unsubscribe via token.** The unsubscribe link in every email uses the subscriber's `unsubscribe_token`. Clicking it sets `status = 'unsubscribed'` and removes them from all lists. This is the safest CAN-SPAM approach. A per-list unsubscribe could be added later if needed.

**Double opt-in is mandatory.** Every new subscriber gets a confirmation email with a link. They're `pending` until they click it. This protects against spam signups and is required in some jurisdictions.

---

## 2 — Subscribe Flow

### Step 1: Form Submission

Any site can POST to the subscribe endpoint:

```
POST /api/subscribe
Content-Type: application/x-www-form-urlencoded

email=forrest@example.com&name=Forrest&list=packstring-launch
```

Or JSON:

```
POST /api/subscribe
Content-Type: application/json

{"email": "forrest@example.com", "name": "Forrest", "list": "packstring-launch"}
```

**Behavior:**
- If email is new: create subscriber (status: pending), generate confirm_token and unsubscribe_token, add to list, send confirmation email
- If email exists and is confirmed: just add to the new list (no re-confirmation needed)
- If email exists and is pending: resend confirmation email
- If email exists and is unsubscribed: reject with message ("This email has been unsubscribed. Contact us to re-subscribe.")

**Response:** Redirect to a configurable thank-you URL, or return JSON for API consumers.

### Step 2: Confirmation Email

```
Subject: Confirm your signup — Packstring

Hey Forrest,

You signed up for updates from Packstring. 
Click below to confirm:

[Confirm my email]
https://broadwave.fireflysoftware.dev/confirm/{confirm_token}

If you didn't sign up, just ignore this email.

— Firefly Software
```

### Step 3: Confirmation

```
GET /confirm/{confirm_token}
```

Sets subscriber status to `confirmed`, sets `confirmed_at`, clears `confirm_token`. Redirects to a configurable "You're in" page.

---

## 3 — Unsubscribe Flow

Every sent email includes an unsubscribe link:

```
https://broadwave.fireflysoftware.dev/unsubscribe/{unsubscribe_token}
```

### One-Click Unsubscribe

```
GET /unsubscribe/{unsubscribe_token}
```

Displays a simple page: "You've been unsubscribed from all Firefly/Packstring emails. If this was a mistake, contact us at [email]."

Sets subscriber status to `unsubscribed`, sets `unsubscribed_at`, removes from all lists.

### List-Unsubscribe Header

Every sent email should include the `List-Unsubscribe` and `List-Unsubscribe-Post` headers for one-click unsubscribe in email clients that support RFC 8058:

```
List-Unsubscribe: <https://broadwave.fireflysoftware.dev/unsubscribe/{token}>
List-Unsubscribe-Post: List-Unsubscribe=One-Click
```

---

## 4 — Sending

### Compose

The admin creates a message tied to a list. The message has:
- Subject line
- Plain text body (required)
- HTML body (optional — if omitted, plain text is sent as-is)

The bodies support simple template variables:
- `{{name}}` — subscriber name (falls back to "there" if empty)
- `{{email}}` — subscriber email
- `{{unsubscribe_url}}` — the subscriber's unsubscribe link

### Send Process

When the admin hits "Send":

1. Status set to `sending`
2. Query all confirmed subscribers on the target list
3. For each subscriber:
   - Create a `send_log` entry (status: queued)
   - Render the template with subscriber-specific variables
   - Send via SMTP
   - Update `send_log` entry (status: sent or failed, with error if applicable)
4. After all sends complete: update message status to `sent`, set `sent_at`, set `sent_count`

**Rate limiting:** Send with a configurable delay between emails (default: 100ms) to avoid triggering SMTP provider rate limits. For a list of 50–500 subscribers, this means sends take 5–50 seconds.

**SMTP configuration:** Stored in a config file or environment variables, not in the database.

```toml
[smtp]
host = "smtp.example.com"
port = 587
username = "hello@fireflysoftware.dev"
password = "..."
tls = true
```

---

## 5 — Admin Interface

### Auth

Same approach as the Packstring availability calendar admin: single username/password, session cookie, bcrypt hash. No roles, no user management.

### Pages

**Dashboard**
- List of all lists with subscriber count (confirmed only)
- Recent messages with status

**List Detail**
- Subscriber table: email, name, status, confirmed date
- Filter by status (confirmed / pending / unsubscribed)
- Manual add subscriber (skips double opt-in — admin override)
- Remove subscriber from list
- Export confirmed emails as CSV

**Compose Message**
- Select target list
- Subject line input
- Plain text body textarea
- HTML body textarea (optional, collapsible)
- Preview button (renders template with sample data)
- Send button with confirmation: "Send to {N} confirmed subscribers on {list name}?"

**Message Detail**
- Subject, body, sent date
- Send stats: total, sent, failed
- Send log table if there were failures

### Tech Stack

- Go + html/template or templ
- HTMX for interactions
- Alpine.js for client-side toggles (preview, confirmation modal)
- Tailwind CSS with Firefly design tokens (this is an internal tool, Firefly-branded)
- SQLite

---

## 6 — Embeddable Form

For use on Packstring, Firefly, and client sites. Two approaches:

### Option A: Plain HTML Form (recommended for server-rendered sites)

```html
<form action="https://broadwave.fireflysoftware.dev/api/subscribe" method="POST">
  <input type="hidden" name="list" value="packstring-launch">
  <input type="hidden" name="redirect" value="https://packstring.dev/thanks/">
  <input type="email" name="email" placeholder="Your email" required>
  <input type="text" name="name" placeholder="Your name (optional)">
  <!-- Honeypot spam protection -->
  <input type="text" name="website" style="display:none" tabindex="-1" autocomplete="off">
  <button type="submit">Keep Me Posted</button>
</form>
```

The form POSTs directly to the mail service. On success, the user is redirected to the `redirect` URL. The honeypot field catches bots.

### Option B: HTMX (for Firefly/Packstring sites)

```html
<form hx-post="https://broadwave.fireflysoftware.dev/api/subscribe" 
      hx-target="#signup-result" 
      hx-swap="innerHTML">
  <input type="hidden" name="list" value="packstring-launch">
  <input type="email" name="email" placeholder="Your email" required>
  <input type="text" name="website" style="display:none" tabindex="-1" autocomplete="off">
  <button type="submit">Keep Me Posted</button>
</form>
<div id="signup-result"></div>
```

The API detects HTMX requests (via `HX-Request` header) and returns an HTML fragment instead of redirecting:

```html
<div class="alert alert--success">
  <div class="alert-title">Check your inbox</div>
  <div class="alert-body">We sent a confirmation link to your email. Click it to confirm your signup.</div>
</div>
```

---

## 7 — Spam Protection

- **Honeypot field** — hidden form field that bots fill out. If `website` field has a value, silently reject.
- **Rate limiting** — max 5 subscribe requests per IP per hour.
- **Double opt-in** — confirmation email prevents fake signups from reaching the confirmed list.
- **No open relay** — the send endpoint is admin-only, behind auth. There's no public API for sending.

---

## 8 — CAN-SPAM / GDPR Compliance

- **Physical address** in every sent email footer (CAN-SPAM requirement)
- **Unsubscribe link** in every sent email, functional within 10 days (CAN-SPAM) — Broadwave does it instantly
- **List-Unsubscribe header** for one-click unsubscribe in email clients
- **Double opt-in** satisfies GDPR consent requirements
- **No data sharing** — subscriber data stays in your SQLite database on your server
- **Export and delete** — admin can export a subscriber's data or delete them entirely (GDPR right to erasure)

Required footer for every sent email:
```
—
Firefly Software
[Physical address]
You received this because you signed up at [site]. 
Unsubscribe: {{unsubscribe_url}}
```

---

## 9 — Development Plan

### Phase 1 — Data Model + Subscribe Flow
- [ ] SQLite schema and migrations
- [ ] `POST /api/subscribe` endpoint with honeypot + rate limiting
- [ ] Confirmation email sending via SMTP
- [ ] `GET /confirm/{token}` endpoint
- [ ] Thank-you and already-subscribed response pages

**Testable checkpoint:** Embed a form on a test page, submit an email, receive confirmation email, click confirm, see subscriber in database as confirmed.

### Phase 2 — Unsubscribe + Admin Foundation
- [ ] `GET /unsubscribe/{token}` endpoint
- [ ] Admin auth (login, session)
- [ ] Admin dashboard (list of lists, subscriber counts)
- [ ] List detail page (subscriber table, filter by status)
- [ ] Manual add/remove subscriber
- [ ] CSV export

**Testable checkpoint:** Admin can log in, see lists, view subscribers, manually add one, and export to CSV. Unsubscribe link works.

### Phase 3 — Compose + Send
- [ ] Compose message form (subject, text body, optional HTML body)
- [ ] Template variable rendering (name, email, unsubscribe_url)
- [ ] Preview functionality
- [ ] Send with rate-limited SMTP delivery
- [ ] Send log with per-subscriber status
- [ ] List-Unsubscribe header on all sent emails
- [ ] CAN-SPAM footer injection

**Testable checkpoint:** Admin can compose a message, preview it, send to a test list of 3 subscribers, and see delivery status per subscriber.

### Phase 4 — Polish + Hardening
- [ ] HTMX response fragments for embedded forms
- [ ] Configurable redirect URLs per list
- [ ] Failed send retry (manual trigger from admin)
- [ ] Bounce handling (mark subscriber if SMTP returns permanent failure)
- [ ] Rate limit tuning for production SMTP provider
- [ ] Backup strategy for SQLite database

**Testable checkpoint:** Full flow works end-to-end on production: signup on packstring.dev → confirmation email → confirmed → admin sends launch email → subscriber receives it with working unsubscribe link.

### Estimated Dev Time

~15–20 hours across the four phases. The subscribe/confirm/unsubscribe plumbing (Phases 1–2) is ~6–8 hours. The compose/send system (Phase 3) is ~6–8 hours. Polish is ~3–4 hours.

---

## 10 — Future Considerations

Things this spec intentionally leaves out that could be added later:

- **Per-list unsubscribe** (stay on other lists when unsubscribing from one)
- **Scheduled sends** (compose now, send at 8 AM Tuesday)
- **Open/click tracking** (adds complexity and privacy concerns — consider carefully)
- **HTML email templates** (a library of pre-styled templates matching Firefly/Packstring brands)
- **API key auth** for external services to subscribe users programmatically
- **Webhook on subscribe/unsubscribe** for integrating with other systems
- **Multiple SMTP providers** (failover if primary is down)
- **Reusable for client sites** — outfitter clients could use this for their seasonal "booking is open" emails, managed through the same admin