package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/saidmuradkhan/cbar-rates/internal/api"
	"github.com/saidmuradkhan/cbar-rates/internal/cbar"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	cache := api.NewCache(cbar.NewClient(), time.Hour)
	srv := api.NewServer(cache)

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, srv))
}
