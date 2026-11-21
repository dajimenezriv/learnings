// Package logger provides the functionality to setup a logger with OTel.
package logger

import (
	"context"
	"log/slog"
	"os"
	"slices"

	"go.opentelemetry.io/otel"

	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func Setup() {
	slog.SetDefault(slog.New(traceHandler{slog.NewTextHandler(os.Stderr, nil)}))

	otel.SetTracerProvider(sdktrace.NewTracerProvider())
	otel.SetTextMapPropagator(propagation.TraceContext{})
}

type attrsKey struct{}

func With(ctx context.Context, args ...any) context.Context {
	var prev []any
	if v, ok := ctx.Value(attrsKey{}).([]any); ok {
		prev = v
	}

	return context.WithValue(ctx, attrsKey{}, append(slices.Clip(prev), args...))
}

type traceHandler struct{ slog.Handler }

func (h traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if args, ok := ctx.Value(attrsKey{}).([]any); ok {
		r.Add(args...)
	}

	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}

	return h.Handler.Handle(ctx, r) //nolint:wrapcheck // transparent decorator; must not alter the wrapped handler's errors
}

func (h traceHandler) WithAttrs(as []slog.Attr) slog.Handler {
	return traceHandler{h.Handler.WithAttrs(as)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{h.Handler.WithGroup(name)}
}
