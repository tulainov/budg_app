package main

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type server struct {
	db         *pgxpool.Pool
	privateKey *rsa.PrivateKey
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" || req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "email, password and display_name are required")
		return
	}
	if req.HouseholdID == "" && req.HouseholdName == "" {
		writeError(w, http.StatusBadRequest, "either household_id or household_name is required")
		return
	}

	ctx := r.Context()

	var household Household
	if req.HouseholdID != "" {
		err := s.db.QueryRow(ctx,
			`SELECT id, name, created_at FROM auth.households WHERE id = $1`,
			req.HouseholdID,
		).Scan(&household.ID, &household.Name, &household.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "household not found")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "lookup household failed")
			return
		}
	} else {
		err := s.db.QueryRow(ctx,
			`INSERT INTO auth.households (name) VALUES ($1) RETURNING id, name, created_at`,
			req.HouseholdName,
		).Scan(&household.ID, &household.Name, &household.CreatedAt)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "create household failed")
			return
		}
	}

	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash password failed")
		return
	}

	var user User
	err = s.db.QueryRow(ctx,
		`INSERT INTO auth.users (household_id, email, password_hash, display_name)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, household_id, email, display_name, created_at`,
		household.ID, req.Email, passwordHash, req.DisplayName,
	).Scan(&user.ID, &user.HouseholdID, &user.Email, &user.DisplayName, &user.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "create user failed")
		return
	}

	token, err := issueToken(s.privateKey, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "issue token failed")
		return
	}

	writeJSON(w, http.StatusCreated, AuthResponse{Token: token, User: user, Household: household})
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	ctx := r.Context()
	var user User
	err := s.db.QueryRow(ctx,
		`SELECT id, household_id, email, password_hash, display_name, created_at
		 FROM auth.users WHERE email = $1`,
		req.Email,
	).Scan(&user.ID, &user.HouseholdID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup user failed")
		return
	}

	if !checkPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	var household Household
	err = s.db.QueryRow(ctx,
		`SELECT id, name, created_at FROM auth.households WHERE id = $1`,
		user.HouseholdID,
	).Scan(&household.ID, &household.Name, &household.CreatedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup household failed")
		return
	}

	token, err := issueToken(s.privateKey, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "issue token failed")
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{Token: token, User: user, Household: household})
}
