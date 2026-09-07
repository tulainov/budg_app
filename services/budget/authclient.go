package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// authClient resolves user_id -> display_name from the auth service, for
// attributing shared transactions to whoever created them (their own JWT
// only tells budget about themselves, not other household members).
//
// This is a genuine runtime dependency on auth being reachable, unlike JWT
// verification (which never calls out to auth). It's kept strictly
// best-effort: any failure here (auth down, timeout, unknown user) just
// means the attribution is omitted, never a failed request — the ledger
// itself must keep working even if this enrichment can't.
type authClient struct {
	baseURL string
	http    *http.Client
}

func newAuthClient(baseURL string) *authClient {
	return &authClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 3 * time.Second},
	}
}

func (c *authClient) lookupDisplayName(ctx context.Context, bearerToken, userID string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/users/"+userID, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	res, err := c.http.Do(req)
	if err != nil {
		return "", false
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", false
	}

	var body struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return "", false
	}
	return body.DisplayName, true
}
