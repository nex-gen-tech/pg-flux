package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UsersHandler struct{ pool *pgxpool.Pool }

func NewUsers(pool *pgxpool.Pool) *UsersHandler { return &UsersHandler{pool: pool} }

func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, handle, email, bio, created_at FROM users ORDER BY created_at`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type row struct {
		ID        int64   `json:"id"`
		Handle    string  `json:"handle"`
		Email     string  `json:"email"`
		Bio       *string `json:"bio"`
		CreatedAt string  `json:"created_at"`
	}
	var out []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.Handle, &r.Email, &r.Bio, &r.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out = append(out, r)
	}
	if out == nil {
		out = []row{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
