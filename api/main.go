package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"atlas/internal/attend"
	"atlas/internal/httpapi"
	"atlas/internal/storage"
	"atlas/internal/store"

	"github.com/redis/go-redis/v9"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	port := env("PORT", "8080")
	dsn := os.Getenv("DATABASE_URL")
	redisAddr := os.Getenv("REDIS_ADDR")
	secret := os.Getenv("JWT_SECRET")
	uploadDir := env("UPLOAD_DIR", "./uploads")

	if len(secret) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 bytes")
	}

	db, err := store.Open(dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("ping redis: %v", err)
	}

	if os.Getenv("SEED") == "true" {
		if err := store.Seed(db); err != nil {
			log.Fatalf("seed: %v", err)
		}
	}

	srv := httpapi.NewServer(
		db,
		attend.NewRedisLimiter(rdb, 10, time.Minute),
		secret,
		storage.LocalStorage{Dir: uploadDir, BaseURL: "/uploads"},
	)
	log.Fatal(http.ListenAndServe(":"+port, srv.Routes()))
}
