package http

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/domain"
	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/service"
)

type Server struct {
	service *service.LedgerService
	db      *sql.DB
	mux     *http.ServeMux
}

func NewServer(svc *service.LedgerService, db *sql.DB) *Server {
	s := &Server{
		service: svc,
		db:      db,
		mux:     http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	var handler http.Handler = s.mux
	handler = LoggingMiddleware(handler)
	handler = RequestIDMiddleware(handler)
	handler = RecoveryMiddleware(handler)
	return handler
}

func (s *Server) registerRoutes() {
	// Health and readiness probes
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)

	// Accounts endpoints
	s.mux.HandleFunc("POST /v1/accounts", s.handleCreateAccount)
	s.mux.HandleFunc("GET /v1/accounts/{id}", s.handleGetAccount)
	s.mux.HandleFunc("GET /v1/accounts/{id}/balance", s.handleGetBalance)
	s.mux.HandleFunc("GET /v1/accounts/{id}/entries", s.handleListEntries)

	// Transactions endpoints
	s.mux.HandleFunc("POST /v1/transactions", s.handleRecordTransaction)
	s.mux.HandleFunc("GET /v1/transactions/{id}", s.handleGetTransaction)
	s.mux.HandleFunc("POST /v1/transactions/{id}/reverse", s.handleReverseTransaction)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := s.db.PingContext(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var req service.CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, domain.ErrInvalidAccountType)
		return
	}

	clientID := r.Header.Get("X-Client-ID")
	if clientID != "" {
		req.ClientID = clientID
	}

	acc, err := s.service.CreateAccount(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, acc)
}

func (s *Server) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, domain.ErrInvalidAccountID)
		return
	}

	acc, err := s.service.GetAccount(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, acc)
}

func (s *Server) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, domain.ErrInvalidAccountID)
		return
	}

	bal, err := s.service.GetAccountBalance(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, bal)
}

func (s *Server) handleListEntries(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, domain.ErrInvalidAccountID)
		return
	}

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			limit = val
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil {
			offset = val
		}
	}

	entries, err := s.service.ListEntries(r.Context(), id, limit, offset)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"account_id": id,
		"entries":    entries,
		"limit":      limit,
		"offset":     offset,
	})
}

type RecordTransactionHTTPBody struct {
	IdempotencyKey string                 `json:"idempotency_key"`
	Description    string                 `json:"description"`
	Entries        []service.EntryRequest `json:"entries"`
}

func (s *Server) handleRecordTransaction(w http.ResponseWriter, r *http.Request) {
	var body RecordTransactionHTTPBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, domain.ErrUnbalancedTransaction)
		return
	}

	// Idempotency key from header takes precedence
	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey != "" {
		body.IdempotencyKey = idemKey
	}

	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		clientID = "default_client"
	}

	req := service.RecordTransactionRequest{
		ClientID:       clientID,
		IdempotencyKey: body.IdempotencyKey,
		Description:    body.Description,
		Entries:        body.Entries,
	}

	tx, isNew, err := s.service.RecordTransaction(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}

	if isNew {
		writeJSON(w, http.StatusCreated, tx)
	} else {
		// Idempotent retry: return 200 OK with original transaction
		writeJSON(w, http.StatusOK, tx)
	}
}

func (s *Server) handleGetTransaction(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, domain.ErrInvalidTransactionID)
		return
	}

	tx, err := s.service.GetTransaction(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, tx)
}

type ReverseTransactionHTTPBody struct {
	IdempotencyKey string `json:"idempotency_key"`
	Description    string `json:"description"`
}

func (s *Server) handleReverseTransaction(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	origTxID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, domain.ErrInvalidTransactionID)
		return
	}

	var body ReverseTransactionHTTPBody
	_ = json.NewDecoder(r.Body).Decode(&body)

	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey != "" {
		body.IdempotencyKey = idemKey
	}

	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		clientID = "default_client"
	}

	req := service.ReverseTransactionRequest{
		ClientID:              clientID,
		IdempotencyKey:        body.IdempotencyKey,
		OriginalTransactionID: origTxID,
		Description:           body.Description,
	}

	tx, isNew, err := s.service.ReverseTransaction(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}

	if isNew {
		writeJSON(w, http.StatusCreated, tx)
	} else {
		writeJSON(w, http.StatusOK, tx)
	}
}
