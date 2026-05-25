package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/nex-gen-tech/pg-flux/examples/go-blog/internal/db"
	"github.com/nex-gen-tech/pg-flux/examples/go-blog/internal/handler"
)

func main() {
	ctx := context.Background()
	pool, err := db.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	posts := handler.NewPosts(pool)
	users := handler.NewUsers(pool)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /posts", posts.List)
	mux.HandleFunc("GET /users", users.List)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
