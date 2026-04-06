package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/ezkahan/fintech-ledger/internal/domain"
	"github.com/ezkahan/fintech-ledger/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AccountHandler struct {
	svc service.AccountService
}

func NewAccountHandler(svc service.AccountService) *AccountHandler {
	return &AccountHandler{svc: svc}
}

type createAccountRequest struct {
	UserID   uuid.UUID `json:"user_id" binding:"required"`
	Currency string    `json:"currency" binding:"required,len=3"`
}

type accountResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
}

type balanceResponse struct {
	AccountID uuid.UUID `json:"account_id"`
	Currency  string    `json:"currency"`
	Balance   int64     `json:"balance"`
}

func toAccountResponse(a domain.Account) accountResponse {
	return accountResponse{
		ID:        a.ID,
		UserID:    a.UserID,
		Currency:  a.Currency,
		CreatedAt: a.CreatedAt,
	}
}

func (h *AccountHandler) Create(c *gin.Context) {
	var req createAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	acc, err := h.svc.CreateAccount(c.Request.Context(), req.UserID, req.Currency)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDuplicateAccount):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrCurrencyNotSupported):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusCreated, toAccountResponse(acc))
}

func (h *AccountHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	acc, err := h.svc.GetAccount(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, toAccountResponse(acc))
}

func (h *AccountHandler) ListByUserID(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	accounts, err := h.svc.ListUserAccounts(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	resp := make([]accountResponse, len(accounts))
	for i, a := range accounts {
		resp[i] = toAccountResponse(a)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AccountHandler) GetBalance(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	money, err := h.svc.GetBalance(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, balanceResponse{
		AccountID: id,
		Currency:  money.Currency(),
		Balance:   money.Amount(),
	})
}

type TransferHandler struct {
	svc service.TransferService
}

func NewTransferHandler(svc service.TransferService) *TransferHandler {
	return &TransferHandler{svc: svc}
}

type transferRequest struct {
	ReferenceID   string    `json:"reference_id" binding:"required"`
	FromAccountID uuid.UUID `json:"from_account_id" binding:"required"`
	ToAccountID   uuid.UUID `json:"to_account_id" binding:"required"`
	Amount        int64     `json:"amount" binding:"required,gt=0"`
	Currency      string    `json:"currency" binding:"required,len=3"`
}

type entryResponse struct {
	ID            uuid.UUID `json:"id"`
	TransactionID uuid.UUID `json:"transaction_id"`
	AccountID     uuid.UUID `json:"account_id"`
	Amount        int64     `json:"amount"`
	Currency      string    `json:"currency"`
	Type          string    `json:"type"`
	CreatedAt     time.Time `json:"created_at"`
}

type transactionResponse struct {
	ID          uuid.UUID       `json:"id"`
	ReferenceID string          `json:"reference_id"`
	Status      string          `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	Entries     []entryResponse `json:"entries"`
}

func toTransactionResponse(tx domain.Transaction) transactionResponse {
	entries := make([]entryResponse, len(tx.Entries))
	for i, e := range tx.Entries {
		entries[i] = entryResponse{
			ID:            e.ID,
			TransactionID: e.TransactionID,
			AccountID:     e.AccountID,
			Amount:        e.Amount.Amount(),
			Currency:      e.Amount.Currency(),
			Type:          string(e.Type),
			CreatedAt:     e.CreatedAt,
		}
	}
	return transactionResponse{
		ID:          tx.ID,
		ReferenceID: tx.ReferenceID,
		Status:      string(tx.Status),
		CreatedAt:   tx.CreatedAt,
		Entries:     entries,
	}
}

func (h *TransferHandler) Transfer(c *gin.Context) {
	var req transferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx, err := h.svc.Transfer(c.Request.Context(), service.TransferRequest{
		ReferenceID:   req.ReferenceID,
		FromAccountID: req.FromAccountID,
		ToAccountID:   req.ToAccountID,
		Amount:        req.Amount,
		Currency:      req.Currency,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDuplicateReference):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrAccountNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrInsufficientFunds):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, domain.ErrCurrencyNotSupported):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusCreated, toTransactionResponse(tx))
}

func (h *TransferHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction id"})
		return
	}

	tx, err := h.svc.GetTransaction(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrTransactionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, toTransactionResponse(tx))
}

func (h *TransferHandler) GetByReference(c *gin.Context) {
	refID := c.Param("ref_id")
	if refID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reference_id is required"})
		return
	}

	tx, err := h.svc.GetTransactionByReference(c.Request.Context(), refID)
	if err != nil {
		if errors.Is(err, domain.ErrTransactionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, toTransactionResponse(tx))
}
