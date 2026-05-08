package likeable

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type rateLimitConfig struct {
	anonymousLimit      int
	anonymousWindow     time.Duration
	authenticatedLimit  int
	authenticatedWindow time.Duration
}

type rateLimitBucket struct {
	count int
	reset time.Time
}

type rateLimitResult struct {
	allowed    bool
	limit      int
	remaining  int
	reset      time.Time
	retryAfter time.Duration
}

type RateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]rateLimitBucket
	config    rateLimitConfig
	now       func() time.Time
	lastSweep time.Time
	redis     *redis.Client
	prefix    string
}

func newRateLimiter(config rateLimitConfig) *RateLimiter {
	if config.anonymousLimit <= 0 {
		config.anonymousLimit = 60
	}
	if config.anonymousWindow <= 0 {
		config.anonymousWindow = time.Minute
	}
	if config.authenticatedLimit <= 0 {
		config.authenticatedLimit = 5000
	}
	if config.authenticatedWindow <= 0 {
		config.authenticatedWindow = time.Hour
	}
	return &RateLimiter{
		buckets: make(map[string]rateLimitBucket),
		config:  config,
		now:     time.Now,
		prefix:  "likeable:rl:",
	}
}

func newRedisRateLimiter(config rateLimitConfig, client *redis.Client) *RateLimiter {
	limiter := newRateLimiter(config)
	limiter.redis = client
	return limiter
}

func (l *RateLimiter) allow(ctx context.Context, key string, limit int, window time.Duration) (rateLimitResult, error) {
	if l.redis != nil {
		return l.allowRedis(ctx, key, limit, window)
	}
	return l.allowMemory(key, limit, window), nil
}

func (l *RateLimiter) allowMemory(key string, limit int, window time.Duration) rateLimitResult {
	now := l.now().UTC()
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.lastSweep.IsZero() || now.Sub(l.lastSweep) > time.Minute {
		for bucketKey, bucket := range l.buckets {
			if !bucket.reset.After(now) {
				delete(l.buckets, bucketKey)
			}
		}
		l.lastSweep = now
	}

	bucket, ok := l.buckets[key]
	if !ok || !bucket.reset.After(now) {
		bucket = rateLimitBucket{reset: now.Add(window)}
	}
	if bucket.count >= limit {
		return rateLimitResult{
			allowed:    false,
			limit:      limit,
			remaining:  0,
			reset:      bucket.reset,
			retryAfter: bucket.reset.Sub(now),
		}
	}
	bucket.count++
	l.buckets[key] = bucket
	return rateLimitResult{
		allowed:   true,
		limit:     limit,
		remaining: max(0, limit-bucket.count),
		reset:     bucket.reset,
	}
}

func (l *RateLimiter) allowRedis(ctx context.Context, key string, limit int, window time.Duration) (rateLimitResult, error) {
	now := l.now().UTC()
	if l.redis == nil {
		return rateLimitResult{}, errors.New("redis rate limiter is not configured")
	}
	redisKey := l.prefix + key
	count, err := l.redis.Incr(ctx, redisKey).Result()
	if err != nil {
		return rateLimitResult{}, err
	}
	if count == 1 {
		if err := l.redis.PExpire(ctx, redisKey, window).Err(); err != nil {
			return rateLimitResult{}, err
		}
	}
	ttl, err := l.redis.PTTL(ctx, redisKey).Result()
	if err != nil || ttl <= 0 {
		ttl = window
	}
	reset := now.Add(ttl)
	remaining := limit - int(count)
	if remaining < 0 {
		remaining = 0
	}
	if int(count) > limit {
		return rateLimitResult{
			allowed:    false,
			limit:      limit,
			remaining:  0,
			reset:      reset,
			retryAfter: ttl,
		}, nil
	}
	return rateLimitResult{
		allowed:   true,
		limit:     limit,
		remaining: remaining,
		reset:     reset,
	}, nil
}

func (s *Server) rateLimiter() *RateLimiter {
	s.limiterOnce.Do(func() {
		if s.limiter == nil {
			s.limiter = newRateLimiter(rateLimitConfig{})
		}
	})
	return s.limiter
}

func (s *Server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		limiter := s.rateLimiter()
		cfg := limiter.config
		key := "ip:" + requestIP(r)
		limit := cfg.anonymousLimit
		window := cfg.anonymousWindow
		nextRequest := r
		if user, err := s.currentUser(r); err == nil && user != nil {
			key = "user:" + user.ID
			limit = cfg.authenticatedLimit
			window = cfg.authenticatedWindow
			nextRequest = r.WithContext(context.WithValue(r.Context(), userContextKey, user))
		}

		result, err := limiter.allow(r.Context(), key, limit, window)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "rate limiter unavailable")
			return
		}
		setRateLimitHeaders(w, result)
		if !result.allowed {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, nextRequest)
	})
}

func setRateLimitHeaders(w http.ResponseWriter, result rateLimitResult) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.reset.Unix(), 10))
	if !result.allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(result.retryAfter)))
	}
}

func retryAfterSeconds(duration time.Duration) int {
	if duration <= 0 {
		return 1
	}
	seconds := int((duration + time.Second - time.Nanosecond) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}

func requestIP(r *http.Request) string {
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		for _, part := range strings.Split(forwardedFor, ",") {
			if ip := normalizeIP(part); ip != "" {
				return ip
			}
		}
	}
	if realIP := normalizeIP(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		if ip := normalizeIP(host); ip != "" {
			return ip
		}
		return host
	}
	if ip := normalizeIP(r.RemoteAddr); ip != "" {
		return ip
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func normalizeIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	if ip := net.ParseIP(value); ip != nil {
		return ip.String()
	}
	return value
}
