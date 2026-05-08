package likeable

import (
	"crypto/tls"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

func redisURLFromEnv() string {
	return firstNonEmpty(osEnv("REDIS_URL"), osEnv("LIKEABLE_REDIS_URL"))
}

func newRedisClient(rawURL string) (*redis.Client, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, nil
	}
	opts, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, err
	}
	return redis.NewClient(opts), nil
}

func asynqRedisOpt(rawURL string) (asynq.RedisClientOpt, bool, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return asynq.RedisClientOpt{}, false, nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return asynq.RedisClientOpt{}, false, err
	}
	if parsed.Scheme != "redis" && parsed.Scheme != "rediss" {
		return asynq.RedisClientOpt{}, false, errors.New("REDIS_URL must use redis:// or rediss://")
	}
	opt := asynq.RedisClientOpt{
		Network: "tcp",
		Addr:    parsed.Host,
	}
	if parsed.User != nil {
		opt.Username = parsed.User.Username()
		opt.Password, _ = parsed.User.Password()
	}
	if strings.Trim(parsed.Path, "/") != "" {
		db, err := strconv.Atoi(strings.Trim(parsed.Path, "/"))
		if err != nil {
			return asynq.RedisClientOpt{}, false, err
		}
		opt.DB = db
	}
	if parsed.Scheme == "rediss" {
		opt.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return opt, true, nil
}

func osEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
