package http

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/domain"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/service"
)

type Server struct {
	svc *service.OrderService
	db  *sql.DB
	mux *http.ServeMux
}

func NewServer(svc *service.OrderService, db *sql.DB) *Server {
	s := &Server{
		svc: svc,
		db:  db,
		mux: http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)

	s.mux.HandleFunc("POST /v1/orders", s.handleCreateOrder)
	s.mux.HandleFunc("GET /v1/orders/{id}", s.handleGetOrder)
	s.mux.HandleFunc("POST /v1/installments/{id}/pay", s.handlePayInstallment)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := s.db.PingContext(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var req service.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request payload")
		return
	}

	clientID := r.Header.Get("X-Client-ID")
	if clientID != "" {
		req.ClientID = clientID
	}

	order, err := s.svc.CreateOrder(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrDownPaymentDeclined) {
			writeError(w, http.StatusPaymentRequired, "DOWN_PAYMENT_DECLINED", err.Error())
			return
		}
		if errors.Is(err, domain.ErrInvalidAmount) || errors.Is(err, domain.ErrInvalidCurrency) || errors.Is(err, domain.ErrEmptyClientID) {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, order)
}

func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "invalid order UUID")
		return
	}

	order, err := s.svc.GetOrder(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "order not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, order)
}

type PayInstallmentRequest struct {
	PaymentMethodToken string `json:"payment_method_token"`
}

func (s *Server) handlePayInstallment(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "invalid installment UUID")
		return
	}

	var req PayInstallmentRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.PaymentMethodToken == "" {
		req.PaymentMethodToken = "pm_card_visa"
	}

	inst, err := s.svc.PayInstallment(r.Context(), id, req.PaymentMethodToken)
	if err != nil {
		if errors.Is(err, domain.ErrInstallmentNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "installment not found")
			return
		}
		if errors.Is(err, domain.ErrInstallmentAlreadyPaid) {
			writeError(w, http.StatusBadRequest, "ALREADY_PAID", err.Error())
			return
		}
		if errors.Is(err, domain.ErrInstallmentPaymentInProgress) {
			writeError(w, http.StatusConflict, "PAYMENT_IN_PROGRESS", err.Error())
			return
		}
		writeError(w, http.StatusPaymentRequired, "PAYMENT_DECLINED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, inst)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
