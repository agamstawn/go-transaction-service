package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

// newRouter wires the HTTP routes together.
// Keeping it separate from main makes the server easy to test.
func newRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	return mux
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           newRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("API listening on port %s", port)
	log.Fatal(server.ListenAndServe())
}
