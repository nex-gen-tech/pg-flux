package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/nex-gen-tech/pg-flux/examples/go-shop/gen"
)

type OrderHandler struct {
	pool *pgxpool.Pool
}

func NewOrderHandler(pool *pgxpool.Pool) *OrderHandler {
	return &OrderHandler{pool: pool}
}

// POST /orders
func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerID int64  `json:"customer_id"`
		Notes      string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var o dbgen.Order
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO public.orders (customer_id, notes)
		 VALUES ($1, $2)
		 RETURNING id, customer_id, status, total, notes, created_at, updated_at`,
		req.CustomerID, req.Notes,
	).Scan(
		&o.ID, &o.CustomerID, &o.Status, &o.Total, &o.Notes, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(o)
}

// POST /orders/:id/items
func (h *OrderHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	var req struct {
		ProductID int64 `json:"product_id"`
		Qty       int32 `json:"qty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Fetch order's created_at for the composite FK
	var orderDate time.Time
	err = h.pool.QueryRow(r.Context(),
		`SELECT created_at FROM public.orders WHERE id = $1`,
		orderID,
	).Scan(&orderDate)
	if err == pgx.ErrNoRows {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get current price from product
	var unitPrice dbgen.PositiveAmount
	err = h.pool.QueryRow(r.Context(),
		`SELECT price FROM public.products WHERE id = $1`,
		req.ProductID,
	).Scan(&unitPrice)
	if err == pgx.ErrNoRows {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var item dbgen.OrderItem
	err = h.pool.QueryRow(r.Context(),
		`INSERT INTO public.order_items (order_id, order_date, product_id, qty, unit_price)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, order_id, order_date, product_id, qty, unit_price`,
		orderID, orderDate, req.ProductID, req.Qty, unitPrice,
	).Scan(
		&item.ID, &item.OrderID, &item.OrderDate, &item.ProductID, &item.Qty, &item.UnitPrice,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

// POST /orders/:id/process
// Calls the process_order stored PROCEDURE via CALL.
// In pgx, CALL is used the same as any other SQL statement via Exec.
func (h *OrderHandler) Process(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	// pgx calls PROCEDURE via CALL — same as Exec for any DML.
	// No special handling needed; pgx sends "CALL public.process_order($1)" as-is.
	_, err = h.pool.Exec(r.Context(),
		`CALL public.process_order($1)`,
		orderID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"order_id": orderID,
		"status":   "confirmed",
	})
}

// GET /orders/:id
func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	var o dbgen.Order
	err = h.pool.QueryRow(r.Context(),
		`SELECT id, customer_id, status, total, notes, created_at, updated_at
		 FROM public.orders WHERE id = $1`,
		orderID,
	).Scan(
		&o.ID, &o.CustomerID, &o.Status, &o.Total, &o.Notes, &o.CreatedAt, &o.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Call calculate_order_total() SQL function to get live total
	var computedTotal string
	err = h.pool.QueryRow(r.Context(),
		`SELECT public.calculate_order_total($1)`,
		orderID,
	).Scan(&computedTotal)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch order items
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, order_id, order_date, product_id, qty, unit_price
		 FROM public.order_items WHERE order_id = $1`,
		orderID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []dbgen.OrderItem{}
	for rows.Next() {
		var item dbgen.OrderItem
		if err := rows.Scan(
			&item.ID, &item.OrderID, &item.OrderDate, &item.ProductID, &item.Qty, &item.UnitPrice,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		items = append(items, item)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"order":          o,
		"items":          items,
		"computed_total": computedTotal,
	})
}
