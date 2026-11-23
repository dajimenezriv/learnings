package auth0

import (
	"errors"
	"io"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"exercise/internal/cli/domain/auth"
	"exercise/internal/cli/domain/auth/authtest"

	"golang.org/x/oauth2"
)

var errDiskFull = errors.New("disk full")

// The bubble's clock starts here, so expiries can be fixed values.
var bubbleStart = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

func storedToken(access, refresh string, expiry time.Time) *oauth2.Token {
	return &oauth2.Token{AccessToken: access, TokenType: "Bearer", RefreshToken: refresh, Expiry: expiry}
}

func storeWith(t *testing.T, tok *oauth2.Token) *authtest.FakeTokenStore {
	t.Helper()
	store := &authtest.FakeTokenStore{}
	if tok != nil {
		require.NoError(t, store.Save(tok))
	}
	return store
}

func callAPI(t *testing.T, client *http.Client) error {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "https://api.example/actors", http.NoBody)
	require.NoError(t, err)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	// Read and close, so net/http's Client.Timeout goroutine stops (synctest would flag it).
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	return resp.Body.Close()
}

func TestHTTPClient_SendsAndRefreshesTokens(t *testing.T) {
	tests := []struct {
		name          string
		stored        *oauth2.Token
		tokens        []reply
		wantAuth      []string
		wantRefreshes int
		wantSaved     *oauth2.Token
	}{
		{
			name:      "valid token is sent as is",
			stored:    storedToken("access-1", "refresh-1", bubbleStart.Add(time.Hour)),
			wantAuth:  []string{"Bearer access-1", "Bearer access-1"},
			wantSaved: storedToken("access-1", "refresh-1", bubbleStart.Add(time.Hour)),
		},
		{
			name:          "expired token is refreshed once and the rotated pair is saved",
			stored:        storedToken("access-1", "refresh-1", bubbleStart.Add(-time.Minute)),
			tokens:        []reply{tokenReply("access-2", "refresh-2")},
			wantAuth:      []string{"Bearer access-2", "Bearer access-2"},
			wantRefreshes: 1,
			wantSaved:     storedToken("access-2", "refresh-2", time.Time{}),
		},
		{
			// x/oauth2 keeps the old refresh token if the response omits it.
			name:          "refresh without a new refresh token keeps the old one",
			stored:        storedToken("access-1", "refresh-1", bubbleStart.Add(-time.Minute)),
			tokens:        []reply{tokenReply("access-2", "")},
			wantAuth:      []string{"Bearer access-2", "Bearer access-2"},
			wantRefreshes: 1,
			wantSaved:     storedToken("access-2", "refresh-1", time.Time{}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := &fakeAuth0{tokens: tt.tokens}
				store := storeWith(t, tt.stored)
				client := NewHTTPClient(newConfig(f), store)

				require.NoError(t, callAPI(t, client))
				require.NoError(t, callAPI(t, client))

				assert.Equal(t, tt.wantAuth, f.apiAuth)
				require.Len(t, f.tokenForms, tt.wantRefreshes)
				for _, form := range f.tokenForms {
					assert.Equal(t, "refresh_token", form.Get("grant_type"))
					assert.Equal(t, "refresh-1", form.Get("refresh_token"))
					assert.Equal(t, "cli-client", form.Get("client_id"))
				}
				saved := store.Saved()
				assert.Equal(t, tt.wantSaved.AccessToken, saved.AccessToken)
				assert.Equal(t, tt.wantSaved.RefreshToken, saved.RefreshToken)
			})
		})
	}
}

func TestHTTPClient_RefreshesWhenTokenExpiresMidSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeAuth0{tokens: []reply{tokenReply("access-2", "refresh-2")}}
		store := storeWith(t, storedToken("access-1", "refresh-1", bubbleStart.Add(time.Hour)))
		client := NewHTTPClient(newConfig(f), store)

		require.NoError(t, callAPI(t, client))
		time.Sleep(2 * time.Hour) // Fake time: instant.
		require.NoError(t, callAPI(t, client))

		assert.Equal(t, []string{"Bearer access-1", "Bearer access-2"}, f.apiAuth)
		assert.Equal(t, "refresh-2", store.Saved().RefreshToken)
	})
}

func TestHTTPClient_Errors(t *testing.T) {
	tests := []struct {
		name          string
		stored        *oauth2.Token
		tokens        []reply
		saveErr       error
		wantErrIs     error
		wantMessage   string
		wantRefreshes int
	}{
		{
			name:      "nothing stored means not logged in",
			wantErrIs: auth.ErrNotLoggedIn,
		},
		{
			name:      "expired without refresh token means session expired, no IdP call",
			stored:    storedToken("access-1", "", bubbleStart.Add(-time.Minute)),
			wantErrIs: auth.ErrSessionExpired,
		},
		{
			name:          "refresh token rejected (idle/absolute lifetime, revoked, reused)",
			stored:        storedToken("access-1", "refresh-1", bubbleStart.Add(-time.Minute)),
			tokens:        []reply{oauthError("invalid_grant")},
			wantErrIs:     auth.ErrSessionExpired,
			wantRefreshes: 1,
		},
		{
			name:          "other refresh failures are not mistaken for an expired session",
			stored:        storedToken("access-1", "refresh-1", bubbleStart.Add(-time.Minute)),
			tokens:        []reply{{status: http.StatusInternalServerError, body: `{"error": "server_error"}`}},
			wantMessage:   "refresh access token",
			wantRefreshes: 1,
		},
		{
			// The old refresh token is already invalid: losing the new one forces a new login.
			name:          "rotated token that can't be saved fails the request",
			stored:        storedToken("access-1", "refresh-1", bubbleStart.Add(-time.Minute)),
			tokens:        []reply{tokenReply("access-2", "refresh-2")},
			saveErr:       errDiskFull,
			wantErrIs:     errDiskFull,
			wantRefreshes: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := &fakeAuth0{tokens: tt.tokens}
				store := storeWith(t, tt.stored)
				store.Err = tt.saveErr

				err := callAPI(t, NewHTTPClient(newConfig(f), store))

				require.Error(t, err)
				if tt.wantErrIs != nil {
					require.ErrorIs(t, err, tt.wantErrIs)
				}
				if tt.wantMessage != "" {
					require.NotErrorIs(t, err, auth.ErrSessionExpired)
					assert.Contains(t, err.Error(), tt.wantMessage)
				}
				assert.Empty(t, f.apiAuth, "the API must not be called without a valid token")
				assert.Len(t, f.tokenForms, tt.wantRefreshes)
			})
		})
	}
}
