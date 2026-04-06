package worker

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	dlqStream      = "ledger:events:dlq"
	dlqCheckEvery  = 30 * time.Second
	dlqIdleTimeout = 2 * time.Minute
	dlqMaxRetries  = 3
)

type DLQProducer struct {
	rdb *redis.Client
}

func NewDLQProducer(rdb *redis.Client) *DLQProducer {
	return &DLQProducer{rdb: rdb}
}

func (d *DLQProducer) Run(ctx context.Context) {
	ticker := time.NewTicker(dlqCheckEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := d.sweep(ctx); err != nil {
				log.Printf("dlq producer: sweep error: %v", err)
			}
		}
	}
}

func (d *DLQProducer) sweep(ctx context.Context) error {
	pending, err := d.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: outboxStream,
		Group:  consumerGroup,
		Idle:   dlqIdleTimeout,
		Start:  "-",
		End:    "+",
		Count:  100,
	}).Result()
	if err != nil {
		return err
	}

	for _, p := range pending {
		if p.RetryCount < dlqMaxRetries {
			continue
		}

		msgs, err := d.rdb.XRangeN(ctx, outboxStream, p.ID, p.ID, 1).Result()
		if err != nil || len(msgs) == 0 {
			continue
		}

		if err := d.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: dlqStream,
			Values: map[string]any{
				"original_id": p.ID,
				"retry_count": p.RetryCount,
				"consumer":    p.Consumer,
				"data":        msgs[0].Values["data"],
			},
		}).Err(); err != nil {
			log.Printf("dlq producer: write to dlq: %v", err)
			continue
		}

		d.rdb.XAck(ctx, outboxStream, consumerGroup, p.ID)
		log.Printf("dlq producer: moved message %s to DLQ after %d retries", p.ID, p.RetryCount)
	}

	return nil
}
