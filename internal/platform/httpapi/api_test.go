package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
)

func TestToHTTPError(t *testing.T) {
	tests := map[string]struct {
		err      error
		wantCode int
	}{
		"client cancelled": {fmt.Errorf("store: %w", context.Canceled), statusClientClosedRequest},
		"deadline":         {fmt.Errorf("store: %w", context.DeadlineExceeded), http.StatusGatewayTimeout},
		"unexpected":       {errors.New("store: error"), http.StatusInternalServerError},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := toHTTPError(t.Context(), tt.err)
			se, ok := errors.AsType[huma.StatusError](err)
			assert.True(t, ok)
			assert.Equal(t, tt.wantCode, se.GetStatus())
			assert.NotContains(t, se.Error(), "store")
		})
	}
}

func TestClientKey(t *testing.T) {
	tests := map[string]struct {
		remoteAddr string
		xRealIP    string
		want       string
	}{
		"remote addr ipv4":    {"203.0.113.7:5000", "", "203.0.113.7"},
		"x-real-ip wins":      {"10.0.0.1:5000", "203.0.113.7", "203.0.113.7"},
		"ipv4-mapped ipv6":    {"10.0.0.1:5000", "::ffff:203.0.113.7", "203.0.113.7"},
		"ipv6 grouped by /64": {"10.0.0.1:5000", "2001:db8:1:2:aaaa::1", "2001:db8:1:2::/64"},
		"same /64 same key":   {"[2001:db8:1:2:bbbb::9]:5000", "", "2001:db8:1:2::/64"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/abc", http.NoBody)
			r.RemoteAddr = tt.remoteAddr
			if tt.xRealIP != "" {
				r.Header.Set("X-Real-IP", tt.xRealIP)
			}
			var got string
			h := middleware.ClientIPFromHeader("X-Real-IP")(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = clientKey(r)
			}))
			h.ServeHTTP(httptest.NewRecorder(), r)
			assert.Equal(t, tt.want, got)
		})
	}
}
