package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/nex-gen-tech/pg-flux/examples/go-events/gen"
)

type EventHandler struct {
	pool *pgxpool.Pool
}

func NewEventHandler(pool *pgxpool.Pool) *EventHandler {
	return &EventHandler{pool: pool}
}

// resolveWorkspaceID looks up a workspace by slug and returns its ID.
func resolveWorkspaceID(r *http.Request, w http.ResponseWriter, pool *pgxpool.Pool, slug string) (int64, bool) {
	var wsID int64
	err := pool.QueryRow(r.Context(), `SELECT id FROM public.workspaces WHERE slug = $1`, slug).Scan(&wsID)
	if err == pgx.ErrNoRows {
		http.Error(w, "workspace not found", http.StatusNotFound)
		return 0, false
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return 0, false
	}
	return wsID, true
}

// POST /workspaces/:slug/events
func (h *EventHandler) Create(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	wsID, ok := resolveWorkspaceID(r, w, h.pool, slug)
	if !ok {
		return
	}

	var req struct {
		Title       string    `json:"title"`
		Description string    `json:"description"`
		StartsAt    time.Time `json:"starts_at"`
		EndsAt      time.Time `json:"ends_at"`
		Location    *string   `json:"location"`
		Capacity    *int32    `json:"capacity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var ev dbgen.Event
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO public.events (workspace_id, title, description, starts_at, ends_at, location, capacity)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, workspace_id, title, description, status, starts_at, ends_at, location, capacity,
		           metadata, tags, deleted_at, created_at, updated_at, title_lower`,
		wsID, req.Title, req.Description, req.StartsAt, req.EndsAt, req.Location, req.Capacity,
	).Scan(
		&ev.ID, &ev.WorkspaceID, &ev.Title, &ev.Description, &ev.Status,
		&ev.StartsAt, &ev.EndsAt, &ev.Location, &ev.Capacity,
		&ev.Metadata, &ev.Tags, &ev.DeletedAt, &ev.CreatedAt, &ev.UpdatedAt, &ev.TitleLower,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ev)
}

// GET /workspaces/:slug/events
func (h *EventHandler) List(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	wsID, ok := resolveWorkspaceID(r, w, h.pool, slug)
	if !ok {
		return
	}

	statusFilter := r.URL.Query().Get("status")
	var rows pgx.Rows
	var err error

	if statusFilter != "" {
		rows, err = h.pool.Query(r.Context(),
			`SELECT id, workspace_id, title, description, status, starts_at, ends_at, location, capacity,
			        metadata, tags, deleted_at, created_at, updated_at, title_lower
			 FROM public.events
			 WHERE workspace_id = $1 AND deleted_at IS NULL AND status = $2
			 ORDER BY starts_at`,
			wsID, statusFilter,
		)
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT id, workspace_id, title, description, status, starts_at, ends_at, location, capacity,
			        metadata, tags, deleted_at, created_at, updated_at, title_lower
			 FROM public.events
			 WHERE workspace_id = $1 AND deleted_at IS NULL
			 ORDER BY starts_at`,
			wsID,
		)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	events := []dbgen.Event{}
	for rows.Next() {
		var ev dbgen.Event
		if err := rows.Scan(
			&ev.ID, &ev.WorkspaceID, &ev.Title, &ev.Description, &ev.Status,
			&ev.StartsAt, &ev.EndsAt, &ev.Location, &ev.Capacity,
			&ev.Metadata, &ev.Tags, &ev.DeletedAt, &ev.CreatedAt, &ev.UpdatedAt, &ev.TitleLower,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		events = append(events, ev)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// GET /workspaces/:slug/events/:id
func (h *EventHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}

	var ev dbgen.Event
	err = h.pool.QueryRow(r.Context(),
		`SELECT id, workspace_id, title, description, status, starts_at, ends_at, location, capacity,
		        metadata, tags, deleted_at, created_at, updated_at, title_lower
		 FROM public.events WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(
		&ev.ID, &ev.WorkspaceID, &ev.Title, &ev.Description, &ev.Status,
		&ev.StartsAt, &ev.EndsAt, &ev.Location, &ev.Capacity,
		&ev.Metadata, &ev.Tags, &ev.DeletedAt, &ev.CreatedAt, &ev.UpdatedAt, &ev.TitleLower,
	)
	if err == pgx.ErrNoRows {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch confirmed count from materialized view (dbgen.EventStat from views.go)
	var stat dbgen.EventStat
	err = h.pool.QueryRow(r.Context(),
		`SELECT event_id, workspace_id, confirmed_count, total_count
		 FROM public.event_stats WHERE event_id = $1`,
		id,
	).Scan(&stat.EventID, &stat.WorkspaceID, &stat.ConfirmedCount, &stat.TotalCount)
	if err != nil && err != pgx.ErrNoRows {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"event": ev,
		"stats": stat,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// PATCH /events/:id
func (h *EventHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}

	var req struct {
		Title    *string `json:"title"`
		Status   *string `json:"status"`
		Capacity *int32  `json:"capacity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var ev dbgen.Event
	err = h.pool.QueryRow(r.Context(),
		`UPDATE public.events
		 SET title    = COALESCE($2, title),
		     status   = COALESCE($3::event_status, status),
		     capacity = COALESCE($4, capacity)
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING id, workspace_id, title, description, status, starts_at, ends_at, location, capacity,
		           metadata, tags, deleted_at, created_at, updated_at, title_lower`,
		id, req.Title, req.Status, req.Capacity,
	).Scan(
		&ev.ID, &ev.WorkspaceID, &ev.Title, &ev.Description, &ev.Status,
		&ev.StartsAt, &ev.EndsAt, &ev.Location, &ev.Capacity,
		&ev.Metadata, &ev.Tags, &ev.DeletedAt, &ev.CreatedAt, &ev.UpdatedAt, &ev.TitleLower,
	)
	if err == pgx.ErrNoRows {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ev)
}

// DELETE /events/:id  (soft delete)
func (h *EventHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}

	res, err := h.pool.Exec(r.Context(),
		`UPDATE public.events SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if res.RowsAffected() == 0 {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /_stats/events/:id  — calls count_confirmed_attendees(event_id)
func (h *EventHandler) Stats(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}

	var count int64
	err = h.pool.QueryRow(r.Context(),
		`SELECT public.count_confirmed_attendees($1)`, id,
	).Scan(&count)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"event_id":          id,
		"confirmed_count":   count,
	})
}
