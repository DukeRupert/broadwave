package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/dukerupert/broadwave/internal/model"
)

type Queries struct {
	DB *sql.DB
}

func NewQueries(db *sql.DB) *Queries {
	return &Queries{DB: db}
}

func (q *Queries) GetListBySlug(ctx context.Context, slug string) (*model.List, error) {
	row := q.DB.QueryRowContext(ctx, `
		SELECT id, slug, name, description, from_name, from_email, created_at
		FROM lists WHERE slug = ?`, slug)

	var l model.List
	var desc sql.NullString
	var createdAt string
	err := row.Scan(&l.ID, &l.Slug, &l.Name, &desc, &l.FromName, &l.FromEmail, &createdAt)
	if err != nil {
		return nil, err
	}
	l.Description = desc.String
	l.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &l, nil
}

func (q *Queries) GetSubscriberByEmail(ctx context.Context, email string) (*model.Subscriber, error) {
	row := q.DB.QueryRowContext(ctx, `
		SELECT id, email, name, status, confirm_token, unsubscribe_token,
		       confirmed_at, unsubscribed_at, created_at
		FROM subscribers WHERE email = ?`, email)
	return scanSubscriber(row)
}

func (q *Queries) GetSubscriberByConfirmToken(ctx context.Context, token string) (*model.Subscriber, error) {
	row := q.DB.QueryRowContext(ctx, `
		SELECT id, email, name, status, confirm_token, unsubscribe_token,
		       confirmed_at, unsubscribed_at, created_at
		FROM subscribers WHERE confirm_token = ?`, token)
	return scanSubscriber(row)
}

func (q *Queries) CreateSubscriber(ctx context.Context, email, name, confirmToken, unsubToken string) (int64, error) {
	result, err := q.DB.ExecContext(ctx, `
		INSERT INTO subscribers (email, name, confirm_token, unsubscribe_token)
		VALUES (?, ?, ?, ?)`, email, name, confirmToken, unsubToken)
	if err != nil {
		return 0, fmt.Errorf("creating subscriber: %w", err)
	}
	return result.LastInsertId()
}

func (q *Queries) AddSubscriberToList(ctx context.Context, listID, subscriberID int64) error {
	_, err := q.DB.ExecContext(ctx, `
		INSERT OR IGNORE INTO list_subscribers (list_id, subscriber_id)
		VALUES (?, ?)`, listID, subscriberID)
	return err
}

func (q *Queries) ConfirmSubscriber(ctx context.Context, token string) error {
	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE subscribers
		SET status = 'confirmed', confirmed_at = datetime('now'), confirm_token = NULL
		WHERE confirm_token = ? AND status = 'pending'`, token)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("no pending subscriber found for token")
	}

	return tx.Commit()
}

func (q *Queries) UpdateConfirmToken(ctx context.Context, subscriberID int64, token string) error {
	_, err := q.DB.ExecContext(ctx, `
		UPDATE subscribers SET confirm_token = ? WHERE id = ?`, token, subscriberID)
	return err
}

func (q *Queries) GetAllListsWithCounts(ctx context.Context) ([]model.ListWithCount, error) {
	rows, err := q.DB.QueryContext(ctx, `
		SELECT l.id, l.slug, l.name, l.description, l.from_name, l.from_email, l.created_at,
		       COUNT(CASE WHEN s.status = 'confirmed' THEN 1 END) AS confirmed_count
		FROM lists l
		LEFT JOIN list_subscribers ls ON l.id = ls.list_id
		LEFT JOIN subscribers s ON ls.subscriber_id = s.id
		GROUP BY l.id
		ORDER BY l.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []model.ListWithCount
	for rows.Next() {
		var lc model.ListWithCount
		var desc sql.NullString
		var createdAt string
		err := rows.Scan(&lc.ID, &lc.Slug, &lc.Name, &desc, &lc.FromName, &lc.FromEmail, &createdAt, &lc.ConfirmedCount)
		if err != nil {
			return nil, err
		}
		lc.Description = desc.String
		lc.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		lists = append(lists, lc)
	}
	return lists, rows.Err()
}

func (q *Queries) GetListByID(ctx context.Context, id int64) (*model.List, error) {
	row := q.DB.QueryRowContext(ctx, `
		SELECT id, slug, name, description, from_name, from_email, created_at
		FROM lists WHERE id = ?`, id)

	var l model.List
	var desc sql.NullString
	var createdAt string
	err := row.Scan(&l.ID, &l.Slug, &l.Name, &desc, &l.FromName, &l.FromEmail, &createdAt)
	if err != nil {
		return nil, err
	}
	l.Description = desc.String
	l.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &l, nil
}

func (q *Queries) GetSubscribersByList(ctx context.Context, listID int64, statusFilter string) ([]model.SubscriberRow, error) {
	query := `
		SELECT s.id, s.email, s.name, s.status, s.unsubscribe_token,
		       ls.subscribed_at, s.confirmed_at, s.unsubscribed_at, s.created_at
		FROM subscribers s
		JOIN list_subscribers ls ON s.id = ls.subscriber_id
		WHERE ls.list_id = ?`
	args := []any{listID}

	if statusFilter != "" {
		query += ` AND s.status = ?`
		args = append(args, statusFilter)
	}
	query += ` ORDER BY ls.subscribed_at DESC`

	rows, err := q.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []model.SubscriberRow
	for rows.Next() {
		var sr model.SubscriberRow
		var name sql.NullString
		var subscribedAt, confirmedAt, unsubscribedAt, createdAt sql.NullString
		err := rows.Scan(&sr.ID, &sr.Email, &name, &sr.Status, &sr.UnsubscribeToken,
			&subscribedAt, &confirmedAt, &unsubscribedAt, &createdAt)
		if err != nil {
			return nil, err
		}
		sr.Name = name.String
		if subscribedAt.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", subscribedAt.String)
			sr.SubscribedAt = &t
		}
		if confirmedAt.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", confirmedAt.String)
			sr.ConfirmedAt = &t
		}
		if unsubscribedAt.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", unsubscribedAt.String)
			sr.UnsubscribedAt = &t
		}
		if createdAt.Valid {
			sr.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt.String)
		}
		subs = append(subs, sr)
	}
	return subs, rows.Err()
}

func (q *Queries) GetRecentMessages(ctx context.Context, limit int) ([]model.MessageSummary, error) {
	rows, err := q.DB.QueryContext(ctx, `
		SELECT m.id, l.name, m.subject, m.status, m.sent_count, m.created_at
		FROM messages m
		JOIN lists l ON m.list_id = l.id
		ORDER BY m.created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []model.MessageSummary
	for rows.Next() {
		var ms model.MessageSummary
		var createdAt string
		err := rows.Scan(&ms.ID, &ms.ListName, &ms.Subject, &ms.Status, &ms.SentCount, &createdAt)
		if err != nil {
			return nil, err
		}
		ms.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		msgs = append(msgs, ms)
	}
	return msgs, rows.Err()
}

func (q *Queries) CreateSubscriberConfirmed(ctx context.Context, email, name, unsubToken string) (int64, error) {
	result, err := q.DB.ExecContext(ctx, `
		INSERT INTO subscribers (email, name, status, unsubscribe_token, confirmed_at)
		VALUES (?, ?, 'confirmed', ?, datetime('now'))`, email, name, unsubToken)
	if err != nil {
		return 0, fmt.Errorf("creating confirmed subscriber: %w", err)
	}
	return result.LastInsertId()
}

func (q *Queries) RemoveSubscriberFromList(ctx context.Context, listID, subscriberID int64) error {
	_, err := q.DB.ExecContext(ctx, `
		DELETE FROM list_subscribers WHERE list_id = ? AND subscriber_id = ?`, listID, subscriberID)
	return err
}

func (q *Queries) ReactivateSubscriber(ctx context.Context, subscriberID int64) error {
	_, err := q.DB.ExecContext(ctx, `
		UPDATE subscribers
		SET status = 'confirmed', unsubscribed_at = NULL, confirmed_at = datetime('now')
		WHERE id = ?`, subscriberID)
	return err
}

func (q *Queries) GetSubscriberByUnsubscribeToken(ctx context.Context, token string) (*model.Subscriber, error) {
	row := q.DB.QueryRowContext(ctx, `
		SELECT id, email, name, status, confirm_token, unsubscribe_token,
		       confirmed_at, unsubscribed_at, created_at
		FROM subscribers WHERE unsubscribe_token = ?`, token)
	return scanSubscriber(row)
}

func (q *Queries) UnsubscribeGlobal(ctx context.Context, subscriberID int64) error {
	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		UPDATE subscribers
		SET status = 'unsubscribed', unsubscribed_at = datetime('now')
		WHERE id = ?`, subscriberID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		DELETE FROM list_subscribers WHERE subscriber_id = ?`, subscriberID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func (q *Queries) ValidateAPIKey(ctx context.Context, rawKey string, listID int64) (bool, error) {
	hash := sha256Hex(rawKey)
	var count int
	err := q.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM api_keys
		WHERE key_hash = ? AND list_id = ? AND revoked_at IS NULL`, hash, listID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (q *Queries) CreateAPIKey(ctx context.Context, listID int64, rawKey, prefix, label string) (int64, error) {
	hash := sha256Hex(rawKey)
	result, err := q.DB.ExecContext(ctx, `
		INSERT INTO api_keys (list_id, key_prefix, key_hash, label)
		VALUES (?, ?, ?, ?)`, listID, prefix, hash, label)
	if err != nil {
		return 0, fmt.Errorf("creating api key: %w", err)
	}
	return result.LastInsertId()
}

func (q *Queries) RevokeAPIKey(ctx context.Context, keyID, listID int64) error {
	result, err := q.DB.ExecContext(ctx, `
		UPDATE api_keys SET revoked_at = datetime('now')
		WHERE id = ? AND list_id = ? AND revoked_at IS NULL`, keyID, listID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("api key not found or already revoked")
	}
	return nil
}

func (q *Queries) GetAPIKeysByList(ctx context.Context, listID int64) ([]model.APIKey, error) {
	rows, err := q.DB.QueryContext(ctx, `
		SELECT id, list_id, key_prefix, label, created_at, revoked_at
		FROM api_keys WHERE list_id = ?
		ORDER BY created_at DESC`, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []model.APIKey
	for rows.Next() {
		var k model.APIKey
		var createdAt string
		var revokedAt sql.NullString
		err := rows.Scan(&k.ID, &k.ListID, &k.KeyPrefix, &k.Label, &createdAt, &revokedAt)
		if err != nil {
			return nil, err
		}
		k.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		if revokedAt.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", revokedAt.String)
			k.RevokedAt = &t
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (q *Queries) CreateMessage(ctx context.Context, listID int64, subject, bodyText, bodyHTML string) (int64, error) {
	result, err := q.DB.ExecContext(ctx, `
		INSERT INTO messages (list_id, subject, body_text, body_html)
		VALUES (?, ?, ?, ?)`, listID, subject, bodyText, bodyHTML)
	if err != nil {
		return 0, fmt.Errorf("creating message: %w", err)
	}
	return result.LastInsertId()
}

func (q *Queries) GetMessageByID(ctx context.Context, id int64) (*model.Message, error) {
	row := q.DB.QueryRowContext(ctx, `
		SELECT id, list_id, subject, body_text, body_html, status, sent_at, sent_count, created_at
		FROM messages WHERE id = ?`, id)

	var m model.Message
	var bodyHTML sql.NullString
	var sentAt sql.NullString
	var createdAt string
	err := row.Scan(&m.ID, &m.ListID, &m.Subject, &m.BodyText, &bodyHTML, &m.Status, &sentAt, &m.SentCount, &createdAt)
	if err != nil {
		return nil, err
	}
	m.BodyHTML = bodyHTML.String
	if sentAt.Valid {
		t, _ := time.Parse("2006-01-02 15:04:05", sentAt.String)
		m.SentAt = &t
	}
	m.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &m, nil
}

func (q *Queries) UpdateMessageStatus(ctx context.Context, id int64, status string) error {
	_, err := q.DB.ExecContext(ctx, `
		UPDATE messages SET status = ? WHERE id = ?`, status, id)
	return err
}

func (q *Queries) SetMessageSent(ctx context.Context, id int64, sentCount int) error {
	_, err := q.DB.ExecContext(ctx, `
		UPDATE messages SET status = 'sent', sent_at = datetime('now'), sent_count = ? WHERE id = ?`, sentCount, id)
	return err
}

func (q *Queries) GetConfirmedSubscribersForList(ctx context.Context, listID int64) ([]model.SubscriberRow, error) {
	return q.GetSubscribersByList(ctx, listID, "confirmed")
}

func (q *Queries) CreateSendLogEntry(ctx context.Context, messageID, subscriberID int64) (int64, error) {
	result, err := q.DB.ExecContext(ctx, `
		INSERT INTO send_log (message_id, subscriber_id)
		VALUES (?, ?)`, messageID, subscriberID)
	if err != nil {
		return 0, fmt.Errorf("creating send log entry: %w", err)
	}
	return result.LastInsertId()
}

func (q *Queries) UpdateSendLogEntry(ctx context.Context, id int64, status, errMsg string) error {
	_, err := q.DB.ExecContext(ctx, `
		UPDATE send_log SET status = ?, sent_at = datetime('now'), error = ? WHERE id = ?`, status, errMsg, id)
	return err
}

func (q *Queries) GetSendLogByMessage(ctx context.Context, messageID int64) ([]model.SendLogEntry, error) {
	rows, err := q.DB.QueryContext(ctx, `
		SELECT sl.id, sl.message_id, sl.subscriber_id, s.email, sl.status, sl.sent_at, sl.error
		FROM send_log sl
		JOIN subscribers s ON sl.subscriber_id = s.id
		WHERE sl.message_id = ?
		ORDER BY sl.id`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.SendLogEntry
	for rows.Next() {
		var e model.SendLogEntry
		var sentAt sql.NullString
		var errMsg sql.NullString
		err := rows.Scan(&e.ID, &e.MessageID, &e.SubscriberID, &e.Email, &e.Status, &sentAt, &errMsg)
		if err != nil {
			return nil, err
		}
		if sentAt.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", sentAt.String)
			e.SentAt = &t
		}
		e.Error = errMsg.String
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (q *Queries) GetMessagesByList(ctx context.Context, listID int64) ([]model.MessageSummary, error) {
	rows, err := q.DB.QueryContext(ctx, `
		SELECT m.id, l.name, m.subject, m.status, m.sent_count, m.created_at
		FROM messages m
		JOIN lists l ON m.list_id = l.id
		WHERE m.list_id = ?
		ORDER BY m.created_at DESC`, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []model.MessageSummary
	for rows.Next() {
		var ms model.MessageSummary
		var createdAt string
		err := rows.Scan(&ms.ID, &ms.ListName, &ms.Subject, &ms.Status, &ms.SentCount, &createdAt)
		if err != nil {
			return nil, err
		}
		ms.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		msgs = append(msgs, ms)
	}
	return msgs, rows.Err()
}

func scanSubscriber(row *sql.Row) (*model.Subscriber, error) {
	var s model.Subscriber
	var name, confirmToken sql.NullString
	var confirmedAt, unsubscribedAt, createdAt sql.NullString

	err := row.Scan(
		&s.ID, &s.Email, &name, &s.Status, &confirmToken, &s.UnsubscribeToken,
		&confirmedAt, &unsubscribedAt, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	s.Name = name.String
	s.ConfirmToken = confirmToken.String
	if confirmedAt.Valid {
		t, _ := time.Parse("2006-01-02 15:04:05", confirmedAt.String)
		s.ConfirmedAt = &t
	}
	if unsubscribedAt.Valid {
		t, _ := time.Parse("2006-01-02 15:04:05", unsubscribedAt.String)
		s.UnsubscribedAt = &t
	}
	if createdAt.Valid {
		s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt.String)
	}

	return &s, nil
}
