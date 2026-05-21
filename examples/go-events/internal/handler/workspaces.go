package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/nex-gen-tech/pg-flux/examples/go-events/gen"
)

type WorkspaceHandler struct {
	pool *pgxpool.Pool
}

func NewWorkspaceHandler(pool *pgxpool.Pool) *WorkspaceHandler {
	return &WorkspaceHandler{pool: pool}
}

// POST /workspaces
func (h *WorkspaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
		Plan string `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Plan == "" {
		req.Plan = "free"
	}

	var ws dbgen.Workspace
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO public.workspaces (slug, name, plan) VALUES ($1, $2, $3)
		 RETURNING id, slug, name, plan`,
		req.Slug, req.Name, req.Plan,
	).Scan(&ws.ID, &ws.Slug, &ws.Name, &ws.Plan)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ws)
}

// GET /workspaces/:slug
func (h *WorkspaceHandler) Get(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var ws dbgen.Workspace
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, slug, name, plan FROM public.workspaces WHERE slug = $1`,
		slug,
	).Scan(&ws.ID, &ws.Slug, &ws.Name, &ws.Plan)
	if err == pgx.ErrNoRows {
		http.Error(w, "workspace not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ws)
}
