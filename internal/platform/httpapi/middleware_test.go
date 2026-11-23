package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

type fakeLimiter struct {
	ok   bool
	wait time.Duration
}

func (f fakeLimiter) Allow(string) (bool, time.Duration) { return f.ok, f.wait }

func TestLimit(t *testing.T) {
	tests := map[string]struct {
		limiter        fakeLimiter
		wantCode       int
		wantRetryAfter string
	}{
		"too many requests":  {fakeLimiter{ok: false, wait: time.Second}, http.StatusTooManyRequests, "1"},
		"rounds retry-after": {fakeLimiter{ok: false, wait: 667 * time.Millisecond}, http.StatusTooManyRequests, "1"},
		"no rate":            {fakeLimiter{ok: false}, http.StatusTooManyRequests, ""},
		"allow":              {fakeLimiter{ok: true}, http.StatusOK, ""},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			m := NewMiddleware(tt.limiter)

			mux := chi.NewRouter()
			api := humachi.New(mux, huma.DefaultConfig("Test", "1.0.0"))
			huma.Register(api, huma.Operation{
				Method:        http.MethodGet,
				Path:          "/",
				DefaultStatus: http.StatusOK,
				Middlewares:   huma.Middlewares{m.Limit("endpoint")},
			}, func(context.Context, *struct{}) (*struct{}, error) {
				return &struct{}{}, nil
			})

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)
			assert.Equal(t, tt.wantRetryAfter, rec.Header().Get("Retry-After"))
		})
	}
}

func TestRecoverer_RecoversPanic(t *testing.T) {
	handler := func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	RecovererMiddleware(http.HandlerFunc(handler)).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
