package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// func NewRedisClient(addr, password string, db int) (*redis.Client, error) {
// 	client := redis.NewClient(&redis.Options{
// 		Addr:     addr,
// 		Password: password,
// 		DB:       db,
// 	})

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()
// 	if err := client.Ping(ctx).Err(); err != nil {
// 		return nil, fmt.Errorf("connecting to redis: %w", err)
// 	}
// 	return client, nil
// }

func NewRedisClient(addr, password string, db int, useTLS bool) (*redis.Client, error) {
	opts := &redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	}

	if useTLS {
		opts.TLSConfig = &tls.Config{}
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connecting to redis: %w", err)
	}

	return client, nil
}