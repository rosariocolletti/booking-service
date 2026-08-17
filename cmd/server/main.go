package main

import (
	"log"
	"net/http"

	"github.com/rosariocolletti/booking-service/internal/api"
	"github.com/rosariocolletti/booking-service/internal/domain"
	"github.com/rosariocolletti/booking-service/internal/events"
)

func main() {
	repo := domain.NewInMemoryRepository()
	publisher := events.NewInMemoryPublisher()
	server := api.NewServer(repo, publisher)

	addr := ":8080"
	log.Printf("booking-service listening on %s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatal(err)
	}
}
