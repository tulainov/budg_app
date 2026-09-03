package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transactionHandler struct {
	db *pgxpool.Pool
}

func validateTransactionRequest(req TransactionRequest) error {
	if req.Scope != "personal" && req.Scope != "shared" {
		return errors.New("scope must be 'personal' or 'shared'")
	}
	if req.AmountCents == 0 {
		return errors.New("amount_cents must be non-zero")
	}
	return nil
}

func (h *transactionHandler) validateCategory(r *http.Request, householdID string, categoryID *string) error {
	if categoryID == nil {
		return nil
	}
	var exists bool
	err := h.db.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM budget.categories WHERE id = $1 AND household_id = $2)`,
		*categoryID, householdID,
	).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return errCategoryNotInHousehold
	}
	return nil
}

var errCategoryNotInHousehold = errors.New("category_id does not belong to your household")

// canMutate implements the ledger rule: personal transactions may only be
// changed by their owner; shared transactions may be changed by any member
// of the household they belong to.
func canMutate(tx Transaction, claims authClaims) bool {
	if tx.HouseholdID != claims.HouseholdID {
		return false
	}
	if tx.Scope == "personal" {
		return tx.UserID == claims.UserID
	}
	return true
}

func scanTransaction(row pgx.Row) (Transaction, error) {
	var t Transaction
	var description *string
	err := row.Scan(&t.ID, &t.HouseholdID, &t.UserID, &t.CategoryID, &t.Scope,
		&t.AmountCents, &t.Currency, &description, &t.OccurredAt, &t.CreatedAt)
	if description != nil {
		t.Description = *description
	}
	return t, err
}

const transactionColumns = `id, household_id, user_id, category_id, scope, amount_cents, currency, description, occurred_at, created_at`

func (h *transactionHandler) create(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())

	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validateTransactionRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Currency == "" {
		req.Currency = "EUR"
	}
	occurredAt := time.Now()
	if req.OccurredAt != nil {
		occurredAt = *req.OccurredAt
	}

	if err := h.validateCategory(r, claims.HouseholdID, req.CategoryID); err != nil {
		if errors.Is(err, errCategoryNotInHousehold) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "validate category failed")
		return
	}

	row := h.db.QueryRow(r.Context(),
		`INSERT INTO budget.transactions
		 (household_id, user_id, category_id, scope, amount_cents, currency, description, occurred_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING `+transactionColumns,
		claims.HouseholdID, claims.UserID, req.CategoryID, req.Scope,
		req.AmountCents, req.Currency, nullableString(req.Description), occurredAt,
	)
	t, err := scanTransaction(row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create transaction failed")
		return
	}

	writeJSON(w, http.StatusCreated, t)
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (h *transactionHandler) list(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "all"
	}

	var (
		rows pgx.Rows
		err  error
	)
	switch scope {
	case "personal":
		rows, err = h.db.Query(r.Context(),
			`SELECT `+transactionColumns+` FROM budget.transactions
			 WHERE household_id = $1 AND scope = 'personal' AND user_id = $2
			 ORDER BY occurred_at DESC`,
			claims.HouseholdID, claims.UserID)
	case "shared":
		rows, err = h.db.Query(r.Context(),
			`SELECT `+transactionColumns+` FROM budget.transactions
			 WHERE household_id = $1 AND scope = 'shared'
			 ORDER BY occurred_at DESC`,
			claims.HouseholdID)
	case "all":
		rows, err = h.db.Query(r.Context(),
			`SELECT `+transactionColumns+` FROM budget.transactions
			 WHERE household_id = $1 AND (scope = 'shared' OR user_id = $2)
			 ORDER BY occurred_at DESC`,
			claims.HouseholdID, claims.UserID)
	default:
		writeError(w, http.StatusBadRequest, "scope must be 'personal', 'shared' or 'all'")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list transactions failed")
		return
	}
	defer rows.Close()

	transactions := []Transaction{}
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan transaction failed")
			return
		}
		transactions = append(transactions, t)
	}

	writeJSON(w, http.StatusOK, transactions)
}

func (h *transactionHandler) fetchVisible(r *http.Request, id string) (Transaction, error) {
	claims := claimsFromContext(r.Context())
	row := h.db.QueryRow(r.Context(),
		`SELECT `+transactionColumns+` FROM budget.transactions WHERE id = $1`, id)
	t, err := scanTransaction(row)
	if err != nil {
		return Transaction{}, err
	}
	if t.HouseholdID != claims.HouseholdID {
		return Transaction{}, pgx.ErrNoRows
	}
	if t.Scope == "personal" && t.UserID != claims.UserID {
		return Transaction{}, pgx.ErrNoRows
	}
	return t, nil
}

func (h *transactionHandler) get(w http.ResponseWriter, r *http.Request) {
	t, err := h.fetchVisible(r, r.PathValue("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "transaction not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "get transaction failed")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *transactionHandler) update(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id := r.PathValue("id")

	existing, err := h.fetchVisible(r, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "transaction not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "get transaction failed")
		return
	}
	if !canMutate(existing, claims) {
		writeError(w, http.StatusForbidden, "not allowed to modify this transaction")
		return
	}

	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validateTransactionRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Currency == "" {
		req.Currency = existing.Currency
	}
	occurredAt := existing.OccurredAt
	if req.OccurredAt != nil {
		occurredAt = *req.OccurredAt
	}
	if req.CategoryID == nil {
		req.CategoryID = existing.CategoryID
	}
	if err := h.validateCategory(r, claims.HouseholdID, req.CategoryID); err != nil {
		if errors.Is(err, errCategoryNotInHousehold) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "validate category failed")
		return
	}

	row := h.db.QueryRow(r.Context(),
		`UPDATE budget.transactions
		 SET category_id = $1, scope = $2, amount_cents = $3, currency = $4, description = $5, occurred_at = $6
		 WHERE id = $7
		 RETURNING `+transactionColumns,
		req.CategoryID, req.Scope, req.AmountCents, req.Currency, nullableString(req.Description), occurredAt, id,
	)
	t, err := scanTransaction(row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update transaction failed")
		return
	}

	writeJSON(w, http.StatusOK, t)
}

func (h *transactionHandler) delete(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id := r.PathValue("id")

	existing, err := h.fetchVisible(r, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "transaction not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "get transaction failed")
		return
	}
	if !canMutate(existing, claims) {
		writeError(w, http.StatusForbidden, "not allowed to delete this transaction")
		return
	}

	if _, err := h.db.Exec(r.Context(), `DELETE FROM budget.transactions WHERE id = $1`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "delete transaction failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
