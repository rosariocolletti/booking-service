// Package events defines domain events and a minimal in-process publisher.
//
// DDD/event-driven note: in a real service extraction, these events would be
// published to a broker (SQS, Kafka, NATS) so other bounded contexts
// (billing, notifications, guest-portal) can react without the Booking
// service knowing they exist. Here the publisher is in-memory to keep the
// demo self-contained, but the interface is the same shape you'd use with
// a real broker — swapping the Publisher implementation is the only change
// needed to go from in-process to distributed.
package events

import (
	"log"
	"sync"
	"time"
)

// Event is the common shape for anything that happened in the domain.
type Event struct {
	Name       string      `json:"name"`
	OccurredAt time.Time   `json:"occurred_at"`
	Payload    interface{} `json:"payload"`
}

type BookingCreatedPayload struct {
	BookingID  string `json:"booking_id"`
	PropertyID string `json:"property_id"`
}

type BookingCancelledPayload struct {
	BookingID string `json:"booking_id"`
	Reason    string `json:"reason,omitempty"`
}

// Publisher is intentionally minimal — a real implementation would satisfy
// this same interface while publishing to an actual message queue.
type Publisher interface {
	Publish(e Event)
}

// InMemoryPublisher logs events and keeps a bounded history — good enough
// for local dev and for demonstrating the event flow without external deps.
type InMemoryPublisher struct {
	mu      sync.Mutex
	History []Event
}

func NewInMemoryPublisher() *InMemoryPublisher {
	return &InMemoryPublisher{History: make([]Event, 0, 100)}
}

func (p *InMemoryPublisher) Publish(e Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e.OccurredAt = time.Now().UTC()
	p.History = append(p.History, e)
	log.Printf("[event] %s payload=%+v", e.Name, e.Payload)
}
