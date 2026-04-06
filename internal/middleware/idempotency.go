package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	idempotencyHeader = "Idempotency-Key"
	idempotencyTTL    = 24 * time.Hour
	redisKeyPrefix    = "idempotency:"
)

type cachedResponse struct {
	Status int    `json:"status"`
	Body   []byte `json:"body"`
}

type responseCapture struct {
	gin.ResponseWriter
	buf    *bytes.Buffer
	status int
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	rc.buf.Write(b)
	return rc.ResponseWriter.Write(b)
}

func (rc *responseCapture) WriteHeader(status int) {
	rc.status = status
	rc.ResponseWriter.WriteHeader(status)
}

func Idempotency(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader(idempotencyHeader)
		if key == "" {
			c.Next()
			return
		}

		redisKey := redisKeyPrefix + key

		val, err := rdb.Get(context.Background(), redisKey).Bytes()
		if err == nil {
			var cached cachedResponse
			if json.Unmarshal(val, &cached) == nil {
				c.Data(cached.Status, "application/json", cached.Body)
				c.Abort()
				return
			}
		}

		rc := &responseCapture{
			ResponseWriter: c.Writer,
			buf:            &bytes.Buffer{},
			status:         http.StatusOK,
		}
		c.Writer = rc
		c.Next()

		cached := cachedResponse{Status: rc.status, Body: rc.buf.Bytes()}
		if data, err := json.Marshal(cached); err == nil {
			rdb.Set(context.Background(), redisKey, data, idempotencyTTL)
		}
	}
}
