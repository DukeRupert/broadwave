package database

import (
	"context"
	"database/sql"
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
