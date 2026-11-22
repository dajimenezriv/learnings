package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"exercise/internal/platform/logger"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type limiter interface {
	Allow(key string) (bool, time.Duration)
}

type Middleware struct {
	l limiter
}

func NewMiddleware(l limiter) *Middleware {
	return &Middleware{l: l}
}

func TraceMiddleware(next http.Handler) http.Handler {
	tracer := otel.Tracer("learnings")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Continue the caller's trace if it sent a traceparent header or start a new one.
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := tracer.Start(ctx, r.Method, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		next.ServeHTTP(w, r.WithContext(ctx))

		// Name by route pattern (/links/{code}).
		// The route is only known after chi has routed.
		if pattern := chi.RouteContext(ctx).RoutePattern(); pattern != "" {
			span.SetName(r.Method + " " + pattern)
		}
	})
}

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		requestID := middleware.GetReqID(r.Context())
		ctx := logger.With(r.Context(),
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path)

		w.Header().Set(middleware.RequestIDHeader, requestID)
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r.WithContext(ctx))

		// Add prometheus.

		slog.InfoContext(ctx, "request",
			"bytes", ww.BytesWritten(),
			"status", ww.Status(),
			"duration", time.Since(start))
	})
}

func RecovererMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func(ctx context.Context) {
			rvr := recover()
			if rvr == nil {
				return
			}

			if err, ok := rvr.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				// We don't recover http.ErrAbortHandler so the response to the client is
				// aborted, this should not be logged.
				panic(rvr)
			}

			slog.ErrorContext(ctx, "panic recovered",
				"panic", fmt.Sprint(rvr),
				"stack", string(debug.Stack()))

			if r.Header.Get("Connection") != "Upgrade" {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}(r.Context())

		next.ServeHTTP(w, r)
	})
}

func TimeoutMiddleware(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (m *Middleware) Limit(endpoint string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, _ := humachi.Unwrap(ctx)
		key := fmt.Sprintf("%s:%s", endpoint, clientKey(r))
		ok, wait := m.l.Allow(key)
		if ok {
			next(ctx)
			return
		}

		if wait > 0 {
			seconds := int(math.Ceil(wait.Seconds()))
			ctx.SetHeader("Retry-After", strconv.Itoa(seconds))
		}
		ctx.SetStatus(http.StatusTooManyRequests)
	}
}
