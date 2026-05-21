package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/nex-gen-tech/pg-flux/examples/go-shop/internal/db"
	"github.com/nex-gen-tech/pg-flux/examples/go-shop/internal/handler"
)

func main() {
	ctx := context.Background()

	pool, err := db.NewPool(ctx)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	ch := handler.NewCustomerHandler(pool)
	ph := handler.NewProductHandler(pool)
	oh := handler.NewOrderHandler(pool)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Customer routes
	r.Post("/customers", ch.Create)
	r.Get("/customers/{id}", ch.Get)

	// Product routes
	r.Post("/products", ph.Create)
	r.Get("/products/search", ph.Search)
	r.Get("/products", ph.List)
	r.Get("/products/{id}/price", ph.GetCurrentPrice)

	// Order routes
	r.Post("/orders", oh.Create)
	r.Post("/orders/{id}/items", oh.AddItem)
	r.Post("/orders/{id}/process", oh.Process)
	r.Get("/orders/{id}", oh.Get)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8014"
	}
	addr := fmt.Sprintf(":%s", port)
	log.Printf("go-shop listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
