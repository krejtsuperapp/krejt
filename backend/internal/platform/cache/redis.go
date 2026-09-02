// Package cache — Redis (ElastiCache cluster mode në AWS, Redis i thjeshtë lokalisht).
// Përdorimi (§42): GEO për shoferët, locks, cache, rate limiting, gjendje kalimtare.
// Kurrë burim autoritativ për të dhëna financiare.
package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"krejt.app/backend/internal/platform/config"
)

func Connect(ctx context.Context, c config.Redis) (redis.UniversalClient, error) {
	opts := &redis.UniversalOptions{
		Addrs:        []string{c.Host},
		Password:     c.Password,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     32,
	}
	if c.TLS {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	// UniversalClient zgjedh vetë: cluster (ElastiCache configuration endpoint) ose një nyje (lokal).
	rdb := redis.NewUniversalClient(opts)
	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := rdb.Ping(pctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping %s: %w", c.Host, err)
	}
	return rdb, nil
}
