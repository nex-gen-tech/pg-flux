package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostsHandler struct{ pool *pgxpool.Pool }

func NewPosts(pool *pgxpool.Pool) *PostsHandler { return &PostsHandler{pool: pool} }

func (h *PostsHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, slug, title, author_handle, published_at FROM published_posts ORDER BY published_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type row struct {
		ID          int64   `json:"id"`
		Slug        string  `json:"slug"`
		Title       string  `json:"title"`
		AuthorHandle string `json:"author_handle"`
		PublishedAt *string `json:"published_at"`
	}
	var out []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.Slug, &r.Title, &r.AuthorHandle, &r.PublishedAt); err != nil {
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
