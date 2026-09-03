package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type categoryHandler struct {
	db *pgxpool.Pool
}

func (h *categoryHandler) create(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())

	var req CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Kind != "income" && req.Kind != "expense" {
		writeError(w, http.StatusBadRequest, "kind must be 'income' or 'expense'")
		return
	}

	var c Category
	err := h.db.QueryRow(r.Context(),
		`INSERT INTO budget.categories (household_id, name, kind)
		 VALUES ($1, $2, $3)
		 RETURNING id, household_id, name, kind, created_at`,
		claims.HouseholdID, req.Name, req.Kind,
	).Scan(&c.ID, &c.HouseholdID, &c.Name, &c.Kind, &c.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			writeError(w, http.StatusConflict, "category already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "create category failed")
		return
	}

	writeJSON(w, http.StatusCreated, c)
}

func (h *categoryHandler) list(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())

	rows, err := h.db.Query(r.Context(),
		`SELECT id, household_id, name, kind, created_at
		 FROM budget.categories WHERE household_id = $1 ORDER BY name`,
		claims.HouseholdID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list categories failed")
		return
	}
	defer rows.Close()

	categories := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.HouseholdID, &c.Name, &c.Kind, &c.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan category failed")
			return
		}
		categories = append(categories, c)
	}

	writeJSON(w, http.StatusOK, categories)
}

func (h *categoryHandler) delete(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id := r.PathValue("id")

	tag, err := h.db.Exec(r.Context(),
		`DELETE FROM budget.categories WHERE id = $1 AND household_id = $2`,
		id, claims.HouseholdID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete category failed")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "category not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
