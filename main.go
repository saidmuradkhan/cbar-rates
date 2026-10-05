package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/saidmuradkhan/cbar-rates/internal/api"
	"github.com/saidmuradkhan/cbar-rates/internal/cbar"
	"github.com/saidmuradkhan/cbar-rates/internal/gate"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	cache := api.NewCache(cbar.NewClient(), time.Hour)
	srv := gate.FromEnv().Wrap(api.NewServer(cache))

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, srv))
}
