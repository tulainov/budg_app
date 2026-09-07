package main

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

type UserSummary struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// handleGetUser resolves a user_id to a display name for cross-service
// attribution (e.g. budget showing who created a shared transaction). Scoped
// to the caller's own household — same 404-not-403 pattern used elsewhere in
// this codebase, so a lookup outside your household doesn't confirm whether
// that user_id exists at all.
func (s *server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id := r.PathValue("id")

	var summary UserSummary
	var householdID string
	err := s.db.QueryRow(r.Context(),
		`SELECT id, display_name, household_id FROM auth.users WHERE id = $1`,
		id,
	).Scan(&summary.ID, &summary.DisplayName, &householdID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup user failed")
		return
	}
	if householdID != claims.HouseholdID {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, summary)
}
