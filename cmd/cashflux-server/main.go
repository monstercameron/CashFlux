package main

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/monstercameron/CashFlux/internal/server"
)

func main() {
	cfg, err := server.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	store, err := server.OpenStore(filepath.Join(cfg.DataDir, "cashflux-server.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	log.Printf("cashflux server listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, server.NewMux(cfg, store)); err != nil {
		log.Fatal(err)
	}
}
