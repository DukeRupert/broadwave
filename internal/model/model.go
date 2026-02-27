package model

import "time"

type SubscriberStatus string

const (
	StatusPending      SubscriberStatus = "pending"
	StatusConfirmed    SubscriberStatus = "confirmed"
	StatusUnsubscribed SubscriberStatus = "unsubscribed"
)

type List struct {
	ID          int64
	Slug        string
	Name        string
	Description string
	FromName    string
	FromEmail   string
	CreatedAt   time.Time
}

type Subscriber struct {
	ID               int64
	Email            string
	Name             string
	Status           SubscriberStatus
	ConfirmToken     string
	UnsubscribeToken string
	ConfirmedAt      *time.Time
	UnsubscribedAt   *time.Time
	CreatedAt        time.Time
}

type ListWithCount struct {
	List
	ConfirmedCount int
}

type SubscriberRow struct {
	ID               int64
	Email            string
	Name             string
	Status           SubscriberStatus
	UnsubscribeToken string
	SubscribedAt     *time.Time
	ConfirmedAt      *time.Time
	UnsubscribedAt   *time.Time
	CreatedAt        time.Time
}

type Message struct {
	ID        int64
	ListID    int64
	Subject   string
	BodyText  string
	BodyHTML  string
	Status    string // draft | sending | sent | failed
	SentAt    *time.Time
	SentCount int
	CreatedAt time.Time
}

type SendLogEntry struct {
	ID           int64
	MessageID    int64
	SubscriberID int64
	Email        string // denormalized for display
	Status       string // queued | sent | failed
	SentAt       *time.Time
	Error        string
}

type MessageSummary struct {
	ID        int64
	ListName  string
	Subject   string
	Status    string
	SentCount int
	CreatedAt time.Time
}

type APIKey struct {
	ID        int64
	ListID    int64
	KeyPrefix string
	Label     string
	CreatedAt time.Time
	RevokedAt *time.Time
}
