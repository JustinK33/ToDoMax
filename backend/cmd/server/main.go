package main

import (
	"log"
	"net/http"
	"os"

	"github.com/justin/todomax/backend/internal/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv, err := server.New()
	if err != nil {
		log.Fatalf("failed to init server: %v", err)
	}
	defer srv.Close()

	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, srv.Routes()); err != nil {
		log.Fatal(err)
	}
}
