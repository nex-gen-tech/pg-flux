package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/nex-gen-tech/pg-flux/examples/go-events/internal/db"
	"github.com/nex-gen-tech/pg-flux/examples/go-events/internal/handler"
)

func main() {
	ctx := context.Background()

	pool, err := db.NewPool(ctx)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	wh := handler.NewWorkspaceHandler(pool)
	eh := handler.NewEventHandler(pool)
	ah := handler.NewAttendeeHandler(pool)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Workspace routes
	r.Post("/workspaces", wh.Create)
	r.Get("/workspaces/{slug}", wh.Get)

	// Event routes (scoped under workspace slug)
	r.Post("/workspaces/{slug}/events", eh.Create)
	r.Get("/workspaces/{slug}/events", eh.List)
	r.Get("/workspaces/{slug}/events/{id}", eh.Get)

	// Event mutation routes (by event ID only)
	r.Patch("/events/{id}", eh.Update)
	r.Delete("/events/{id}", eh.Delete)

	// Attendee routes
	r.Post("/events/{id}/attendees", ah.Register)
	r.Get("/events/{id}/attendees", ah.List)
	r.Patch("/events/{id}/attendees/{userId}", ah.UpdateStatus)

	// Stats route — calls count_confirmed_attendees(event_id) SQL function
	r.Get("/_stats/events/{id}", eh.Stats)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8013"
	}
	addr := fmt.Sprintf(":%s", port)
	log.Printf("go-events listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
