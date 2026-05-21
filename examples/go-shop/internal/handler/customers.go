package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/nex-gen-tech/pg-flux/examples/go-shop/gen"
)

type CustomerHandler struct {
	pool *pgxpool.Pool
}

func NewCustomerHandler(pool *pgxpool.Pool) *CustomerHandler {
	return &CustomerHandler{pool: pool}
}

// POST /customers
func (h *CustomerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		FullName string `json:"full_name"`
		Tier     string `json:"tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Tier == "" {
		req.Tier = "standard"
	}

	var c dbgen.Customer
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO public.customers (email, full_name, tier)
		 VALUES ($1, $2, $3::public.customer_tier)
		 RETURNING id, email, full_name, tier, shipping_addr, metadata, created_at, updated_at, phone`,
		req.Email, req.FullName, req.Tier,
	).Scan(
		&c.ID, &c.Email, &c.FullName, &c.Tier,
		&c.ShippingAddr, &c.Metadata, &c.CreatedAt, &c.UpdatedAt, &c.Phone,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(c)
}

// GET /customers/:id
func (h *CustomerHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid customer id", http.StatusBadRequest)
		return
	}

	var c dbgen.Customer
	err = h.pool.QueryRow(r.Context(),
		`SELECT id, email, full_name, tier, shipping_addr, metadata, created_at, updated_at, phone
		 FROM public.customers WHERE id = $1`,
		id,
	).Scan(
		&c.ID, &c.Email, &c.FullName, &c.Tier,
		&c.ShippingAddr, &c.Metadata, &c.CreatedAt, &c.UpdatedAt, &c.Phone,
	)
	if err == pgx.ErrNoRows {
		http.Error(w, "customer not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}
