# Broadwave

A minimal, self-hosted email list management app built in Go + SQLite. Handles subscribe/unsubscribe with double opt-in, list management, and sending emails to lists via Postmark.

## Setup

### 1. Build

```bash
go build -o broadwave .
```

### 2. Configure

Create a `config.toml` file (or pass a custom path with `-config`):

```toml
[app]
listen_addr = ":8090"
base_url = "https://your-domain.com"
cors_origins = ["https://your-frontend.com"]  # optional — origins allowed to call /api/subscribe via fetch

[database]
path = "broadwave.db"
backup_dir = "./backups"  # optional — enables daily SQLite backups

[postmark]
server_token = "your-postmark-server-token"
message_stream = "outbound"

[subscribe]
default_redirect = "/"

[admin]
username = "admin"
password_hash = "$2a$10$..."  # bcrypt hash
session_ttl = "24h"

[compliance]
physical_address = "123 Main St, City, ST 12345"
```

Generate a password hash:

```bash
htpasswd -nbBC 10 "" 'your-password' | cut -d: -f2
```

### 3. Run

```bash
./broadwave -config config.toml
```

The database is created and migrated automatically on first run.

## API

### Subscribe

```
POST /api/subscribe
```

Adds an email to a list. A confirmation email is sent automatically (double opt-in). The subscriber must click the confirmation link before they receive any messages.

**Authentication:** Every request requires an API key scoped to the target list. Create API keys in the admin UI under each list's detail page.

Pass the API key using one of:
- Form/JSON field: `api_key`
- Header: `Authorization: Bearer <key>`

**Request formats:**

Form-encoded:

```bash
curl -X POST https://your-domain.com/api/subscribe \
  -d "email=user@example.com" \
  -d "name=Jane Doe" \
  -d "list=newsletter" \
  -d "api_key=bw_k_..."
```

JSON:

```bash
curl -X POST https://your-domain.com/api/subscribe \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer bw_k_..." \
  -d '{"email": "user@example.com", "name": "Jane Doe", "list": "newsletter"}'
```

HTMX (returns HTML fragment instead of redirect):

```html
<form hx-post="/api/subscribe" hx-swap="outerHTML">
  <input type="hidden" name="api_key" value="bw_k_..." />
  <input type="hidden" name="list" value="newsletter" />
  <input type="email" name="email" placeholder="you@example.com" required />
  <input type="text" name="name" placeholder="Your name" />
  <!-- honeypot — hide with CSS -->
  <input type="text" name="website" style="display:none" tabindex="-1" autocomplete="off" />
  <button type="submit">Subscribe</button>
</form>
```

**Parameters:**

| Field | Required | Description |
|-------|----------|-------------|
| `email` | yes | Subscriber email address |
| `list` | yes | List slug (set when creating a list in admin) |
| `api_key` | yes | API key for the target list (or use `Authorization: Bearer` header) |
| `name` | no | Subscriber name |
| `redirect` | no | URL to redirect to on success (form submissions only, overrides `default_redirect`) |
| `website` | no | Honeypot field — if filled, the request is silently discarded |

**Responses:**

| Scenario | JSON | Form |
|----------|------|------|
| Success (new subscriber) | `200` `{"message": "Check your inbox for a confirmation link."}` | `303` redirect |
| Success (already confirmed) | `200` `{"message": "You've been added to the list."}` | `303` redirect |
| Missing/invalid API key | `401` / `403` `{"error": "..."}` | `401` / `403` plain text |
| Invalid email or list | `400` `{"error": "..."}` | `400` plain text |
| Rate limited | `429` `{"error": "Too many requests..."}` | `429` plain text |

Cross-origin requests (detected via `Origin` header) always receive JSON responses instead of redirects.

Rate limit: 5 requests per IP per hour.

**CORS:** To call `/api/subscribe` from a different domain using `fetch()`, add the origin to `cors_origins` in your config:

```toml
[app]
cors_origins = ["https://your-frontend.com", "https://www.your-frontend.com"]
```

The endpoint accepts `application/x-www-form-urlencoded`, `multipart/form-data`, and `application/json` request bodies.

### Confirm

```
GET /confirm/{token}
```

Clicked by the subscriber from their confirmation email. Activates the subscription. Returns an HTML success page.

### Unsubscribe

```
GET /unsubscribe/{token}
```

Globally unsubscribes the email from all lists. Included automatically in every sent email via the `List-Unsubscribe` header and CAN-SPAM footer. Returns an HTML confirmation page.

## Admin

The admin UI is at `/admin/` and requires login. From there you can:

- **Lists** — view all lists, create new lists, subscriber counts, and create API keys
- **List detail** — view/filter subscribers, add subscribers manually, export CSV, manage API keys
- **Compose** — write and preview messages with template variables (`{{name}}`, `{{email}}`, `{{unsubscribe_url}}`)
- **Messages** — view send status, delivery stats (sent/failed/bounced), and per-subscriber send logs

## Backups

Set `database.backup_dir` in your config to enable automatic SQLite backups:

```toml
[database]
backup_dir = "./backups"
```

When enabled, Broadwave runs a backup on startup and every 24 hours using `VACUUM INTO`, which is safe on an active WAL-mode database. Backup files older than 7 days are pruned automatically.
