package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb     *redis.Client
	logger  *slog.Logger
	enabled bool
}

func New(addr, password string, logger *slog.Logger) *Client {
	if addr == "" {
		return &Client{logger: logger}
	}
	return &Client{
		rdb: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       0,
		}),
		logger:  logger,
		enabled: true,
	}
}

func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

func (c *Client) GetJSON(ctx context.Context, key string, dest interface{}) (bool, error) {
	if c == nil || !c.enabled {
		return false, nil
	}
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		c.warn("redis cache get failed", err)
		return false, err
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Client) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if c == nil || !c.enabled {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := c.rdb.Set(ctx, key, raw, ttl).Err(); err != nil {
		c.warn("redis cache set failed", err)
		return err
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, key string) {
	if c == nil || !c.enabled {
		return
	}
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		c.warn("redis delete failed", err)
	}
}

func (c *Client) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if c == nil || !c.enabled || limit <= 0 {
		return true, nil
	}
	count, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		c.warn("redis rate limit increment failed", err)
		return true, err
	}
	if count == 1 {
		_ = c.rdb.Expire(ctx, key, window).Err()
	}
	return count <= int64(limit), nil
}

func (c *Client) warn(message string, err error) {
	if c != nil && c.logger != nil {
		c.logger.Warn(message, "error", err)
	}
}
