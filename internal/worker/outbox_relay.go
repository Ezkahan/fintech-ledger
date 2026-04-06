package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/ezkahan/fintech-ledger/internal/repository"
	"github.com/redis/go-redis/v9"
)

const (
	outboxStream    = "ledger:events"
	outboxPollLimit = 50
	outboxPollEvery = 5 * time.Second
)

type OutboxRelay struct {
	outboxRepo repository.OutboxRepository
	rdb        *redis.Client
}

func NewOutboxRelay(outboxRepo repository.OutboxRepository, rdb *redis.Client) *OutboxRelay {
	return &OutboxRelay{outboxRepo: outboxRepo, rdb: rdb}
}

func (r *OutboxRelay) Run(ctx context.Context) {
	ticker := time.NewTicker(outboxPollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.relay(ctx); err != nil {
				log.Printf("outbox relay error: %v", err)
			}
		}
	}
}

func (r *OutboxRelay) relay(ctx context.Context) error {
	events, err := r.outboxRepo.GetUnprocessed(ctx, outboxPollLimit)
	if err != nil {
		return err
	}

	for _, e := range events {
		payload, err := json.Marshal(map[string]any{
			"id":           e.ID.String(),
			"aggregate_id": e.AggregateID.String(),
			"event_type":   e.EventType,
			"payload":      string(e.Payload),
			"created_at":   e.CreatedAt.Unix(),
		})
		if err != nil {
			log.Printf("outbox relay: marshal event %s: %v", e.ID, err)
			continue
		}

		if err := r.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: outboxStream,
			Values: map[string]any{"data": string(payload)},
		}).Err(); err != nil {
			log.Printf("outbox relay: publish event %s: %v", e.ID, err)
			continue
		}

		if err := r.outboxRepo.MarkProcessed(ctx, e.ID); err != nil {
			log.Printf("outbox relay: mark processed %s: %v", e.ID, err)
		}
	}

	return nil
}
