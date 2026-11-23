package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5/middleware"
)

// statusClientClosedRequest is nginx's non-standard status for a client that
// disconnected before getting a response. Nobody receives it; it's for the logs.
const statusClientClosedRequest = 499

func toHTTPError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return huma.NewError(statusClientClosedRequest, "")
	case errors.Is(err, context.DeadlineExceeded):
		return huma.Error504GatewayTimeout("")
	default:
		slog.ErrorContext(ctx, "unexpected error", "err", err)
		return huma.Error500InternalServerError("")
	}
}

// clientKey identifies a client for rate limiting. IPv6 clients are grouped by
// their /64, since a single user usually controls the whole prefix.
func clientKey(r *http.Request) string {
	addr := middleware.GetClientIPAddr(r.Context())
	if !addr.IsValid() {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		if addr, err = netip.ParseAddr(ip); err != nil {
			return ip
		}
		addr = addr.Unmap()
	}
	if addr.Is4() {
		return addr.String()
	}
	prefix, err := addr.Prefix(64)
	if err != nil {
		return addr.String()
	}
	return prefix.String()
}
