package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"github.com/cs2browser-net/steam-ingress/config"
	"github.com/cs2browser-net/steam-ingress/fetcher"
	"github.com/cs2browser-net/steam-ingress/geoip"
	"github.com/cs2browser-net/steam-ingress/logger"
	"github.com/cs2browser-net/steam-ingress/store"
)

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func envInt(name string, def, lo, hi int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v >= lo && v <= hi {
		return v
	}
	return def
}

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load("data/config.json")
	if err != nil {
		logger.Fatalf("Config: %v", err)
	}

	cacheKey := os.Getenv("SERVERS_KEY")
	if cacheKey == "" {
		logger.Fatalf("SERVERS_KEY is not set")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		logger.Fatalf("Database: %v", err)
	}
	defer db.Close()

	redisOpts, err := redis.ParseURL(os.Getenv("CACHE_URL"))
	if err != nil {
		logger.Fatalf("CACHE_URL: %v", err)
	}
	cache := redis.NewClient(redisOpts)
	defer cache.Close()

	geo := geoip.New(
		net.JoinHostPort(envOr("GEOIP_HOST", "127.0.0.1"), strconv.Itoa(envInt("GEOIP_PORT", 8081, 1, 65535))),
		envInt("GEOIP_POOL_SIZE", 12, 1, 1024),
		time.Duration(envInt("GEOIP_REQUEST_TIMEOUT_MS", 2500, 1, 1<<31-1))*time.Millisecond,
	)

	st := store.New(db, cache, cacheKey, geo)

	f, err := fetcher.New(cfg, st, geo)
	if err != nil {
		logger.Fatalf("Config: %v", err)
	}

	logger.Printf("Updating cache...")
	if err := st.WarmCache(ctx); err != nil {
		logger.Fatalf("Updating cache: %v", err)
	}
	logger.Printf("Updated cache.")

	f.Run(ctx)
	logger.Printf("Shutting down.")
}
