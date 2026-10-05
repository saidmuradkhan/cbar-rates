package main

import (
	"log"
	"net/http"
	"os"

	"github.com/saidmuradkhan/cbar-rates/internal/api"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, api.NewServer()))
}
