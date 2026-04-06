package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ezkahan/fintech-ledger/internal/domain"
	"github.com/ezkahan/fintech-ledger/internal/handler"
	"github.com/ezkahan/fintech-ledger/internal/repository"
	"github.com/ezkahan/fintech-ledger/internal/service"
	"github.com/ezkahan/fintech-ledger/internal/testhelper"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newRouter(t *testing.T) (*gin.Engine, repository.AccountRepository, repository.TransactionRepository) {
	t.Helper()
	pool := testhelper.NewPool(t)
	testhelper.TruncateTables(t, pool)
	rdb := testhelper.NewRedis(t)

	accRepo := repository.NewAccountRepository(pool)
	txRepo := repository.NewTransactionRepository(pool)
	accountSvc := service.NewAccountService(accRepo)
	transferSvc := service.NewTransferService(accRepo, txRepo)
	accountH := handler.NewAccountHandler(accountSvc)
	transferH := handler.NewTransferHandler(transferSvc)
	r := handler.NewRouter(accountH, transferH, rdb)
	return r, accRepo, txRepo
}

func postJSON(t *testing.T, r *gin.Engine, path string, body any, headers ...map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	for _, h := range headers {
		for k, v := range h {
			req.Header.Set(k, v)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func getJSON(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ── Account endpoints ─────────────────────────────────────────────────────────

func TestHandler_CreateAccount(t *testing.T) {
	r, _, _ := newRouter(t)

	w := postJSON(t, r, "/v1/accounts", map[string]any{
		"user_id":  uuid.New(),
		"currency": "USD",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201 got %d: %s", w.Code, w.Body)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["currency"] != "USD" {
		t.Fatalf("unexpected currency: %v", resp["currency"])
	}
}

func TestHandler_CreateAccount_Duplicate(t *testing.T) {
	r, _, _ := newRouter(t)
	userID := uuid.New()

	postJSON(t, r, "/v1/accounts", map[string]any{"user_id": userID, "currency": "USD"})
	w := postJSON(t, r, "/v1/accounts", map[string]any{"user_id": userID, "currency": "USD"})

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409 got %d", w.Code)
	}
}

func TestHandler_GetAccount(t *testing.T) {
	r, accRepo, _ := newRouter(t)

	acc := domain.NewAccount(uuid.New(), "EUR")
	accRepo.Create(context.Background(), acc) //nolint:errcheck

	w := getJSON(t, r, fmt.Sprintf("/v1/accounts/%s", acc.ID))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body)
	}
}

func TestHandler_GetAccount_NotFound(t *testing.T) {
	r, _, _ := newRouter(t)
	w := getJSON(t, r, fmt.Sprintf("/v1/accounts/%s", uuid.New()))
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d", w.Code)
	}
}

func TestHandler_ListUserAccounts(t *testing.T) {
	r, accRepo, _ := newRouter(t)
	userID := uuid.New()

	for _, cur := range []string{"USD", "EUR"} {
		accRepo.Create(context.Background(), domain.NewAccount(userID, cur)) //nolint:errcheck
	}

	w := getJSON(t, r, fmt.Sprintf("/v1/users/%s/accounts", userID))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body)
	}

	var resp []map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 2 {
		t.Fatalf("want 2 accounts got %d", len(resp))
	}
}

func TestHandler_GetBalance(t *testing.T) {
	r, accRepo, _ := newRouter(t)
	acc := domain.NewAccount(uuid.New(), "USD")
	accRepo.Create(context.Background(), acc) //nolint:errcheck

	w := getJSON(t, r, fmt.Sprintf("/v1/accounts/%s/balance", acc.ID))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d: %s", w.Code, w.Body)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["balance"].(float64) != 0 {
		t.Fatalf("expected zero balance")
	}
}

// ── Transfer endpoints ────────────────────────────────────────────────────────

func fundAccount(t *testing.T, accRepo repository.AccountRepository, txRepo repository.TransactionRepository, accountID uuid.UUID, amount int64, currency string) {
	t.Helper()
	bankAcc := domain.NewAccount(uuid.New(), currency)
	accRepo.Create(context.Background(), bankAcc) //nolint:errcheck
	money := domain.MustNewMoney(amount, currency)
	entries := []domain.Entry{
		{AccountID: bankAcc.ID, Amount: money, Type: domain.EntryDebit},
		{AccountID: accountID, Amount: money, Type: domain.EntryCredit},
	}
	tx, _ := domain.NewTransaction(uuid.NewString(), entries)
	tx.Commit()                              //nolint:errcheck
	txRepo.Create(context.Background(), tx) //nolint:errcheck
}

func TestHandler_Transfer(t *testing.T) {
	r, accRepo, txRepo := newRouter(t)

	accA := domain.NewAccount(uuid.New(), "USD")
	accB := domain.NewAccount(uuid.New(), "USD")
	accRepo.Create(context.Background(), accA) //nolint:errcheck
	accRepo.Create(context.Background(), accB) //nolint:errcheck
	fundAccount(t, accRepo, txRepo, accA.ID, 50_000, "USD")

	w := postJSON(t, r, "/v1/transfers", map[string]any{
		"reference_id":    "http-pay-001",
		"from_account_id": accA.ID,
		"to_account_id":   accB.ID,
		"amount":          10_000,
		"currency":        "USD",
	}, map[string]string{"Idempotency-Key": "http-pay-001"})

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201 got %d: %s", w.Code, w.Body)
	}
}

func TestHandler_Transfer_Idempotency(t *testing.T) {
	r, accRepo, txRepo := newRouter(t)
	rdb := testhelper.NewRedis(t)
	rdb.FlushDB(context.Background()) //nolint:errcheck

	accA := domain.NewAccount(uuid.New(), "USD")
	accB := domain.NewAccount(uuid.New(), "USD")
	accRepo.Create(context.Background(), accA) //nolint:errcheck
	accRepo.Create(context.Background(), accB) //nolint:errcheck
	fundAccount(t, accRepo, txRepo, accA.ID, 50_000, "USD")

	body := map[string]any{
		"reference_id":    "http-idem-001",
		"from_account_id": accA.ID,
		"to_account_id":   accB.ID,
		"amount":          1_000,
		"currency":        "USD",
	}
	header := map[string]string{"Idempotency-Key": "http-idem-001"}

	w1 := postJSON(t, r, "/v1/transfers", body, header)
	w2 := postJSON(t, r, "/v1/transfers", body, header)

	if w1.Code != http.StatusCreated {
		t.Fatalf("first: want 201 got %d: %s", w1.Code, w1.Body)
	}
	if w2.Code != http.StatusCreated {
		t.Fatalf("second (idempotent): want 201 got %d: %s", w2.Code, w2.Body)
	}
	if w1.Body.String() != w2.Body.String() {
		t.Fatal("idempotent responses must be identical")
	}
}

func TestHandler_Transfer_InsufficientFunds(t *testing.T) {
	r, accRepo, _ := newRouter(t)

	accA := domain.NewAccount(uuid.New(), "USD")
	accB := domain.NewAccount(uuid.New(), "USD")
	accRepo.Create(context.Background(), accA) //nolint:errcheck
	accRepo.Create(context.Background(), accB) //nolint:errcheck

	w := postJSON(t, r, "/v1/transfers", map[string]any{
		"reference_id":    "http-insuf",
		"from_account_id": accA.ID,
		"to_account_id":   accB.ID,
		"amount":          1,
		"currency":        "USD",
	})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 got %d: %s", w.Code, w.Body)
	}
}

func TestHandler_GetTransaction(t *testing.T) {
	r, accRepo, txRepo := newRouter(t)

	accA := domain.NewAccount(uuid.New(), "USD")
	accB := domain.NewAccount(uuid.New(), "USD")
	accRepo.Create(context.Background(), accA) //nolint:errcheck
	accRepo.Create(context.Background(), accB) //nolint:errcheck
	fundAccount(t, accRepo, txRepo, accA.ID, 10_000, "USD")

	w := postJSON(t, r, "/v1/transfers", map[string]any{
		"reference_id":    "http-gettx",
		"from_account_id": accA.ID,
		"to_account_id":   accB.ID,
		"amount":          500,
		"currency":        "USD",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("transfer: want 201 got %d", w.Code)
	}

	var tx map[string]any
	json.NewDecoder(w.Body).Decode(&tx)
	txID := tx["id"].(string)

	w2 := getJSON(t, r, fmt.Sprintf("/v1/transactions/%s", txID))
	if w2.Code != http.StatusOK {
		t.Fatalf("get tx: want 200 got %d", w2.Code)
	}
}

func TestHandler_Healthz(t *testing.T) {
	pool := testhelper.NewPool(t)
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	accRepo := repository.NewAccountRepository(pool)
	txRepo := repository.NewTransactionRepository(pool)
	accountSvc := service.NewAccountService(accRepo)
	transferSvc := service.NewTransferService(accRepo, txRepo)
	r := handler.NewRouter(handler.NewAccountHandler(accountSvc), handler.NewTransferHandler(transferSvc), rdb)

	w := getJSON(t, r, "/healthz")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", w.Code)
	}
}
