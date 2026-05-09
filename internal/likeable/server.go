package likeable

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fibegg/likeable/internal/store"
	"github.com/hibiken/asynq"
)

type Server struct {
	store       *store.Store
	config      RuntimeConfig
	http        *http.Client
	recovering  sync.Map
	refreshing  sync.Map
	email       emailSender
	jobs        *JobSystem
	limiter     *RateLimiter
	limiterOnce sync.Once
}

const (
	maxMessageUploadBytes = 80 << 20
	maxMessageAttachments = 5
)

func Run() error {
	role := runtimeRole()
	cfg := RuntimeConfig{
		Addr:           env("ADDR", ":8080"),
		BaseURL:        strings.TrimRight(env("BASE_URL", "http://localhost:8080"), "/"),
		DatabasePath:   env("DATABASE_PATH", "./data/likeable.db"),
		AdminEmail:     normalizeEmail(os.Getenv("ADMIN_EMAIL")),
		RedisURL:       redisURLFromEnv(),
		DevAuth:        os.Getenv("LIKEABLE_DEV_AUTH") == "1",
		BootstrapToken: strings.TrimSpace(os.Getenv("LIKEABLE_BOOTSTRAP_TOKEN")),
		WebDir:         os.Getenv("LIKEABLE_WEB_DIR"),
	}
	appStore, err := store.Open(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer appStore.Close()
	server := &Server{store: appStore, config: cfg, http: &http.Client{Timeout: 30 * time.Second}}
	var redisOpt asynq.RedisClientOpt
	redisConfigured := false
	if redisClient, err := newRedisClient(cfg.RedisURL); err != nil {
		return err
	} else if redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := redisClient.Ping(ctx).Err(); err != nil {
			cancel()
			_ = redisClient.Close()
			return fmt.Errorf("redis unavailable: %w", err)
		}
		cancel()
		defer redisClient.Close()
		server.limiter = newRedisRateLimiter(rateLimitConfig{}, redisClient)
	}
	if opt, ok, err := asynqRedisOpt(cfg.RedisURL); err != nil {
		return err
	} else if ok {
		redisOpt = opt
		redisConfigured = true
	}
	if !redisConfigured {
		if role == "worker" {
			return fmt.Errorf("REDIS_URL is required for worker role")
		}
		log.Printf("REDIS_URL is not configured; using in-memory rate limiting and inline background work")
	} else if role == "web" {
		server.jobs = newJobClient(redisOpt)
		defer server.jobs.Close()
	} else {
		server.jobs = newJobSystem(redisOpt, server)
		defer server.jobs.Close()
	}
	if role == "worker" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		server.startRecurringJobs(ctx)
		log.Printf("likeable worker listening for jobs")
		return server.jobs.Run(ctx)
	}
	if role == "all" && server.jobs != nil && server.jobs.server != nil {
		server.jobs.Start()
	}
	log.Printf("likeable %s listening on %s", role, cfg.Addr)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if role == "all" && server.jobs != nil && server.jobs.server != nil {
		server.startRecurringJobs(ctx)
	}
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func runtimeRole() string {
	role := strings.ToLower(strings.TrimSpace(os.Getenv("LIKEABLE_ROLE")))
	if len(os.Args) > 1 {
		role = strings.ToLower(strings.TrimSpace(os.Args[1]))
	}
	switch role {
	case "", "all":
		return "all"
	case "web", "server", "http":
		return "web"
	case "worker", "jobs", "job":
		return "worker"
	default:
		log.Fatalf("unknown likeable role %q; use all, web, or worker", role)
		return "all"
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
