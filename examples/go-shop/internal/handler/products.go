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

type ProductHandler struct {
	pool *pgxpool.Pool
}

func NewProductHandler(pool *pgxpool.Pool) *ProductHandler {
	return &ProductHandler{pool: pool}
}

// POST /products
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Sku        string   `json:"sku"`
		Name       string   `json:"name"`
		Price      string   `json:"price"` // numeric as string
		CategoryID *int32   `json:"category_id"`
		Tags       []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var p dbgen.Product
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO public.products (sku, name, price, category_id, tags)
		 VALUES ($1, $2, $3::public.positive_amount, $4, $5)
		 RETURNING id, category_id, sku, name, description, status, price, weight_grams,
		           tags, attributes, search_vector, created_at, updated_at, cost`,
		req.Sku, req.Name, req.Price, req.CategoryID, req.Tags,
	).Scan(
		&p.ID, &p.CategoryID, &p.Sku, &p.Name, &p.Description, &p.Status, &p.Price,
		&p.WeightGrams, &p.Tags, &p.Attributes, &p.SearchVector, &p.CreatedAt, &p.UpdatedAt, &p.Cost,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

// GET /products/search?q=
// Uses search_vector @@ to_tsquery for full-text search.
func (h *ProductHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "q query param required", http.StatusBadRequest)
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, category_id, sku, name, description, status, price, weight_grams,
		        tags, attributes, search_vector, created_at, updated_at, cost
		 FROM public.products
		 WHERE search_vector @@ plainto_tsquery('english', $1)
		 ORDER BY id`,
		q,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	products := []dbgen.Product{}
	for rows.Next() {
		var p dbgen.Product
		if err := rows.Scan(
			&p.ID, &p.CategoryID, &p.Sku, &p.Name, &p.Description, &p.Status, &p.Price,
			&p.WeightGrams, &p.Tags, &p.Attributes, &p.SearchVector, &p.CreatedAt, &p.UpdatedAt, &p.Cost,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		products = append(products, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

// GET /products
// Lists active products from the active_products view (uses dbgen.ActiveProduct).
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, sku, name, price, tags, search_vector FROM public.active_products ORDER BY id`,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	products := []dbgen.ActiveProduct{}
	for rows.Next() {
		var p dbgen.ActiveProduct
		if err := rows.Scan(
			&p.ID, &p.Sku, &p.Name, &p.Price, &p.Tags, &p.SearchVector,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		products = append(products, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

// GET /products/:id/price
// Returns the current active price from price_rules (valid_during contains now()).
func (h *ProductHandler) GetCurrentPrice(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	var pr dbgen.PriceRule
	err = h.pool.QueryRow(r.Context(),
		`SELECT id, product_id, price, valid_during::text, label
		 FROM public.price_rules
		 WHERE product_id = $1 AND valid_during @> now()
		 ORDER BY id DESC
		 LIMIT 1`,
		id,
	).Scan(&pr.ID, &pr.ProductID, &pr.Price, &pr.ValidDuring, &pr.Label)
	if err == pgx.ErrNoRows {
		// Fall back to base product price
		var basePrice dbgen.PositiveAmount
		var updatedAt time.Time
		err = h.pool.QueryRow(r.Context(),
			`SELECT price, updated_at FROM public.products WHERE id = $1`,
			id,
		).Scan(&basePrice, &updatedAt)
		if err == pgx.ErrNoRows {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"product_id": id,
			"price":      basePrice,
			"source":     "base",
		})
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"product_id":   pr.ProductID,
		"price":        pr.Price,
		"valid_during": pr.ValidDuring,
		"label":        pr.Label,
		"source":       "price_rule",
	})
}
