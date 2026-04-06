package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	consumerGroup = "ledger-consumers"
	consumerName  = "consumer-1"
	consumerBlock = 5 * time.Second
	consumerCount = 10
)

type Consumer struct {
	rdb *redis.Client
}

func NewConsumer(rdb *redis.Client) *Consumer {
	return &Consumer{rdb: rdb}
}

func (c *Consumer) Run(ctx context.Context) {
	if err := c.ensureGroup(ctx); err != nil {
		log.Printf("consumer: create group: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := c.poll(ctx); err != nil {
				log.Printf("consumer: poll error: %v", err)
				time.Sleep(time.Second)
			}
		}
	}
}

func (c *Consumer) ensureGroup(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, outboxStream, consumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

func (c *Consumer) poll(ctx context.Context) error {
	streams, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: consumerName,
		Streams:  []string{outboxStream, ">"},
		Count:    consumerCount,
		Block:    consumerBlock,
	}).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}

	for _, stream := range streams {
		for _, msg := range stream.Messages {
			if err := c.process(ctx, msg); err != nil {
				log.Printf("consumer: process message %s: %v", msg.ID, err)
				continue
			}
			c.rdb.XAck(ctx, outboxStream, consumerGroup, msg.ID)
		}
	}

	return nil
}

func (c *Consumer) process(ctx context.Context, msg redis.XMessage) error {
	data, ok := msg.Values["data"].(string)
	if !ok {
		return nil
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return err
	}

	log.Printf("consumer: processed event type=%s aggregate_id=%s",
		event["event_type"], event["aggregate_id"])

	return nil
}
