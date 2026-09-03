package main

import "time"

type Category struct {
	ID          string    `json:"id"`
	HouseholdID string    `json:"household_id"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateCategoryRequest struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type Transaction struct {
	ID          string    `json:"id"`
	HouseholdID string    `json:"household_id"`
	UserID      string    `json:"user_id"`
	CategoryID  *string   `json:"category_id,omitempty"`
	Scope       string    `json:"scope"`
	AmountCents int64     `json:"amount_cents"`
	Currency    string    `json:"currency"`
	Description string    `json:"description,omitempty"`
	OccurredAt  time.Time `json:"occurred_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type TransactionRequest struct {
	CategoryID  *string    `json:"category_id,omitempty"`
	Scope       string     `json:"scope"`
	AmountCents int64      `json:"amount_cents"`
	Currency    string     `json:"currency,omitempty"`
	Description string     `json:"description,omitempty"`
	OccurredAt  *time.Time `json:"occurred_at,omitempty"`
}
