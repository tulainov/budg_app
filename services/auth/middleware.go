package main

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const claimsContextKey contextKey = "claims"

type authClaims struct {
	UserID      string
	HouseholdID string
}

// requireAuth verifies a request's bearer token against this same service's
// own signing key. auth never needs a separate public key file for this —
// an *rsa.PrivateKey already carries its public half in-memory
// (privateKey.PublicKey), so no extra Secret or mount is required just to
// let auth authenticate requests to its own endpoints.
func requireAuth(privateKey *rsa.PrivateKey, next http.HandlerFunc) http.HandlerFunc {
	publicKey := &privateKey.PublicKey
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		tokenString, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || tokenString == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return publicKey, nil
		})
		if err != nil || !token.Valid {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		sub, _ := claims.GetSubject()
		householdID, _ := claims["household_id"].(string)
		if sub == "" || householdID == "" {
			writeError(w, http.StatusUnauthorized, "token missing required claims")
			return
		}

		ac := authClaims{UserID: sub, HouseholdID: householdID}
		ctx := context.WithValue(r.Context(), claimsContextKey, ac)
		next(w, r.WithContext(ctx))
	}
}

func claimsFromContext(ctx context.Context) authClaims {
	ac, _ := ctx.Value(claimsContextKey).(authClaims)
	return ac
}
