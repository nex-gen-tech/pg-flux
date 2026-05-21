package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/nex-gen-tech/pg-flux/examples/go-events/gen"
)

type AttendeeHandler struct {
	pool *pgxpool.Pool
}

func NewAttendeeHandler(pool *pgxpool.Pool) *AttendeeHandler {
	return &AttendeeHandler{pool: pool}
}

// POST /events/:id/attendees
func (h *AttendeeHandler) Register(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	eventID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}

	var req struct {
		UserID int64  `json:"user_id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Status == "" {
		req.Status = "invited"
	}

	// DEFERRABLE FK: insert inside a transaction to allow deferred constraint check.
	// The attendees_user_id_fkey is DEFERRABLE INITIALLY DEFERRED — validated at commit.
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	var att dbgen.Attendee
	err = tx.QueryRow(r.Context(),
		`INSERT INTO public.attendees (event_id, user_id, status)
		 VALUES ($1, $2, $3::attendee_status)
		 ON CONFLICT (event_id, user_id) DO UPDATE SET status = EXCLUDED.status
		 RETURNING event_id, user_id, status, registered_at`,
		eventID, req.UserID, req.Status,
	).Scan(&att.EventID, &att.UserID, &att.Status, &att.RegisteredAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		// Deferred FK violation surfaces here if user_id doesn't exist.
		http.Error(w, "attendee registration failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Refresh the materialized view so event_stats is current.
	// In production you'd do this asynchronously; here we do it inline for simplicity.
	h.pool.Exec(r.Context(), `REFRESH MATERIALIZED VIEW CONCURRENTLY public.event_stats`) //nolint:errcheck

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(att)
}

// GET /events/:id/attendees
func (h *AttendeeHandler) List(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	eventID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT a.event_id, a.user_id, a.status, a.registered_at
		 FROM public.attendees a
		 WHERE a.event_id = $1
		 ORDER BY a.registered_at`,
		eventID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	attendees := []dbgen.Attendee{}
	for rows.Next() {
		var att dbgen.Attendee
		if err := rows.Scan(&att.EventID, &att.UserID, &att.Status, &att.RegisteredAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		attendees = append(attendees, att)
	}

	// Also check for an unknown event
	if len(attendees) == 0 {
		var exists bool
		h.pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM public.events WHERE id = $1)`, eventID).Scan(&exists) //nolint:errcheck
		if !exists {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(attendees)
}

// updateAttendeeStatus is a helper for changing attendee status.
func (h *AttendeeHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	eventID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}
	userIDStr := chi.URLParam(r, "userId")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var att dbgen.Attendee
	err = h.pool.QueryRow(r.Context(),
		`UPDATE public.attendees SET status = $3::attendee_status
		 WHERE event_id = $1 AND user_id = $2
		 RETURNING event_id, user_id, status, registered_at`,
		eventID, userID, req.Status,
	).Scan(&att.EventID, &att.UserID, &att.Status, &att.RegisteredAt)
	if err == pgx.ErrNoRows {
		http.Error(w, "attendee not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(att)
}
