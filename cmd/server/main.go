package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"exercise/internal/platform/httpapi"

	"github.com/caarlos0/env/v11"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type config struct {
	DatabaseURL string `env:"DATABASE_URL,required,notEmpty"`
}

func main() {
	if err := dispatch(os.Args[1:]); err != nil {
		log.Printf("❌ %v", err)
		os.Exit(1)
	}
}

func dispatch(args []string) error {
	if len(args) == 0 {
		return run()
	}
	switch args[0] {
	case "openapi":
		return printOpenAPI()
	case "migrate":
		return migrateUp()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func loadConfig() (config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return config{}, fmt.Errorf("load .env: %w", err)
	}
	cfg, err := env.ParseAs[config]()
	if err != nil {
		return config{}, fmt.Errorf("parse config: %w", err)
	}
	// We pool client-side (pgxpool) and migrations take a session-level advisory
	// lock, which breaks under PgBouncer transaction mode. Use the direct endpoint.
	// https://neon.com/docs/connect/connection-pooling
	if strings.Contains(cfg.DatabaseURL, "-pooler") {
		return config{}, errors.New("DATABASE_URL must use the direct Neon endpoint, not -pooler")
	}
	return cfg, nil
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create DB pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping DB: %w", err)
	}
	log.Println("✅ Connected to Postgres")

	app := setupApplication(pool)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           registerHTTPHandlers(app),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Println("🚀 Server running on :8080")
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("listen server: %w", err)
		}
		return nil
	case <-ctx.Done():
		log.Println("🛑 Shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}
	return nil
}

func registerHTTPHandlers(app *application) chi.Router {
	r := chi.NewRouter()
	r.Use(
		middleware.Heartbeat("/healthz"),
		middleware.RequestID,
		// Assumes the server is only reachable through the edge proxy, which
		// overwrites X-Real-IP with the address of the connection it accepted.
		middleware.ClientIPFromHeader("X-Real-IP"),
		httpapi.TraceMiddleware,
		httpapi.LoggerMiddleware,
		httpapi.RecovererMiddleware,
		// Load shedding, not rate limiting: caps in-flight requests across all
		// clients and rejects the rest straight away. 503 instead of the default
		// 429, since the server is overloaded, not the client misbehaving.
		middleware.ThrottleWithOpts(middleware.ThrottleOpts{
			Limit:        1000,
			StatusCode:   http.StatusServiceUnavailable,
			RetryAfterFn: func(bool) time.Duration { return time.Second },
		}),
		middleware.RequestSize(1<<20),
		httpapi.TimeoutMiddleware(10*time.Second),
	)

	registerOperations(humachi.New(r, httpapi.Config()), app)
	return r
}

func registerOperations(api huma.API, app *application) {
	app.GetActorsHTTP.Register(api)
	app.CreateActorHTTP.Register(api)
	app.UpdateActorHTTP.Register(api)
	app.DeleteActorHTTP.Register(api)
	app.GetAccountsHTTP.Register(api)
	app.CreateAccountHTTP.Register(api)
}
