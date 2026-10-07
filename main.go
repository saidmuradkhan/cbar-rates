package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/saidmuradkhan/cbar-rates/internal/api"
	"github.com/saidmuradkhan/cbar-rates/internal/cbar"
	"github.com/saidmuradkhan/cbar-rates/internal/gate"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	cache := api.NewCache(cbar.NewClient(), time.Hour)
	srv := api.LogRequests(slog.Default(), gate.FromEnv().Wrap(api.NewServer(cache)))

	slog.Info("server starting", "port", port)
	if err := http.ListenAndServe(":"+port, srv); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
