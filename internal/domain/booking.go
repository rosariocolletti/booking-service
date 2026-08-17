// Package domain contains the core Booking aggregate.
//
// DDD note: Booking is the aggregate root. All state changes to a booking
// go through its methods (Cancel, Confirm) rather than being set directly
// from outside — this is what keeps business invariants (e.g. "a cancelled
// booking can't be re-confirmed") enforced in one place instead of scattered
// across handlers.
package domain

import (
	"errors"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusConfirmed Status = "confirmed"
	StatusCancelled Status = "cancelled"
)

var (
	ErrInvalidDateRange   = errors.New("check-out must be after check-in")
	ErrAlreadyCancelled   = errors.New("booking is already cancelled")
	ErrCannotConfirm      = errors.New("only a pending booking can be confirmed")
	ErrCannotCancel       = errors.New("a cancelled booking cannot be cancelled again")
)

// Booking is the aggregate root. Its zero value is never valid on its own;
// use NewBooking to construct one so invariants are checked at creation time.
type Booking struct {
	ID        string
	GuestName string
	PropertyID string
	CheckIn   time.Time
	CheckOut  time.Time
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewBooking constructs a Booking, enforcing the core invariant that
// check-out must be after check-in. This is where an event-driven system
// would emit a BookingCreated event — see internal/events.
func NewBooking(id, guestName, propertyID string, checkIn, checkOut time.Time) (*Booking, error) {
	if !checkOut.After(checkIn) {
		return nil, ErrInvalidDateRange
	}
	now := time.Now().UTC()
	return &Booking{
		ID:         id,
		GuestName:  guestName,
		PropertyID: propertyID,
		CheckIn:    checkIn,
		CheckOut:   checkOut,
		Status:     StatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// Confirm transitions a pending booking to confirmed.
func (b *Booking) Confirm() error {
	if b.Status != StatusPending {
		return ErrCannotConfirm
	}
	b.Status = StatusConfirmed
	b.UpdatedAt = time.Now().UTC()
	return nil
}

// Cancel transitions a booking to cancelled, from any non-cancelled state.
func (b *Booking) Cancel() error {
	if b.Status == StatusCancelled {
		return ErrCannotCancel
	}
	b.Status = StatusCancelled
	b.UpdatedAt = time.Now().UTC()
	return nil
}
