package handler

import (
	"net/http"

	"github.com/ezkahan/fintech-ledger/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

func NewRouter(
	accountHandler *AccountHandler,
	transferHandler *TransferHandler,
	rdb *redis.Client,
) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Metrics())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	v1 := r.Group("/v1")

	accounts := v1.Group("/accounts")
	{
		accounts.POST("", accountHandler.Create)
		accounts.GET("/:id", accountHandler.GetByID)
		accounts.GET("/:id/balance", accountHandler.GetBalance)
	}

	users := v1.Group("/users")
	{
		users.GET("/:user_id/accounts", accountHandler.ListByUserID)
	}

	transfers := v1.Group("/transfers")
	transfers.Use(middleware.Idempotency(rdb))
	{
		transfers.POST("", transferHandler.Transfer)
	}

	transactions := v1.Group("/transactions")
	{
		transactions.GET("/:id", transferHandler.GetByID)
		transactions.GET("/reference/:ref_id", transferHandler.GetByReference)
	}

	return r
}
