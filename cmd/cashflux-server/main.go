package main

import (
	"log"
	"net/http"

	"github.com/monstercameron/CashFlux/internal/server"
)

func main() {
	cfg, err := server.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("cashflux server listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, server.NewMux(cfg)); err != nil {
		log.Fatal(err)
	}
}
