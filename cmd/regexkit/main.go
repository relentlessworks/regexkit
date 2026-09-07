package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/relentlessworks/regexkit/internal/api"
	"github.com/relentlessworks/regexkit/internal/auth"
	"github.com/relentlessworks/regexkit/internal/config"
	"github.com/relentlessworks/regexkit/internal/store"
)

func main() {
	cfg := config.Load()

	a := auth.New(cfg.Secret)
	s := store.New(cfg.DB)
	server := api.New(a, s)

	log.Printf("regexkit starting on %s (db=%s)", cfg.Addr, cfg.DB)

	http.Handle("/", server.Routes())
	if err := http.ListenAndServe(cfg.Addr, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
	fmt.Println("done")
}
