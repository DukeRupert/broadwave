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
