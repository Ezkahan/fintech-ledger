package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ezkahan/fintech-ledger/config"
	"github.com/ezkahan/fintech-ledger/internal/repository"
	"github.com/ezkahan/fintech-ledger/internal/worker"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()

	pool, err := pgxpool.New(context.Background(), cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("postgres ping: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       cfg.RedisDB,
	})
	defer rdb.Close()

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("redis ping: %v", err)
	}

	outboxRepo := repository.NewOutboxRepository(pool)

	relay := worker.NewOutboxRelay(outboxRepo, rdb)
	consumer := worker.NewConsumer(rdb)
	dlq := worker.NewDLQProducer(rdb)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go relay.Run(ctx)
	go consumer.Run(ctx)
	go dlq.Run(ctx)

	log.Println("worker: running")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	cancel()
	log.Println("worker: stopped")
}
