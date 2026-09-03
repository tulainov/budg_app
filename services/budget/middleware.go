package main

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const claimsContextKey contextKey = "claims"

type authClaims struct {
	UserID      string
	HouseholdID string
	Email       string
	DisplayName string
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	key, err := jwt.ParseRSAPublicKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	return key, nil
}

// requireAuth verifies the JWT locally against the auth service's public
// key, no network call to the auth service needed per request.
func requireAuth(publicKey *rsa.PublicKey, next http.HandlerFunc) http.HandlerFunc {
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
		email, _ := claims["email"].(string)
		displayName, _ := claims["display_name"].(string)
		if sub == "" || householdID == "" {
			writeError(w, http.StatusUnauthorized, "token missing required claims")
			return
		}

		ac := authClaims{UserID: sub, HouseholdID: householdID, Email: email, DisplayName: displayName}
		ctx := context.WithValue(r.Context(), claimsContextKey, ac)
		next(w, r.WithContext(ctx))
	}
}

func claimsFromContext(ctx context.Context) authClaims {
	ac, _ := ctx.Value(claimsContextKey).(authClaims)
	return ac
}
