package domain

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("booking not found")

// Repository abstracts storage so the API layer never depends on how
// bookings are persisted. Swapping InMemoryRepository for a Postgres-backed
// one later doesn't touch the API or domain layers at all.
type Repository interface {
	Save(b *Booking) error
	FindByID(id string) (*Booking, error)
	List() ([]*Booking, error)
}

type InMemoryRepository struct {
	mu       sync.RWMutex
	bookings map[string]*Booking
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{bookings: make(map[string]*Booking)}
}

func (r *InMemoryRepository) Save(b *Booking) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bookings[b.ID] = b
	return nil
}

func (r *InMemoryRepository) FindByID(id string) (*Booking, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.bookings[id]
	if !ok {
		return nil, ErrNotFound
	}
	return b, nil
}

func (r *InMemoryRepository) List() ([]*Booking, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Booking, 0, len(r.bookings))
	for _, b := range r.bookings {
		out = append(out, b)
	}
	return out, nil
}
