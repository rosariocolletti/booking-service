package domain

import (
	"testing"
	"time"
)

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNewBooking_ValidRange(t *testing.T) {
	b, err := NewBooking("bk_1", "Anna", "prop_1", mustDate("2026-09-10"), mustDate("2026-09-14"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Status != StatusPending {
		t.Errorf("expected status pending, got %s", b.Status)
	}
}

func TestNewBooking_InvalidRange(t *testing.T) {
	_, err := NewBooking("bk_1", "Anna", "prop_1", mustDate("2026-09-14"), mustDate("2026-09-10"))
	if err != ErrInvalidDateRange {
		t.Errorf("expected ErrInvalidDateRange, got %v", err)
	}
}

func TestConfirm_FromPending_Succeeds(t *testing.T) {
	b, _ := NewBooking("bk_1", "Anna", "prop_1", mustDate("2026-09-10"), mustDate("2026-09-14"))
	if err := b.Confirm(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Status != StatusConfirmed {
		t.Errorf("expected confirmed, got %s", b.Status)
	}
}

func TestConfirm_AlreadyConfirmed_Fails(t *testing.T) {
	b, _ := NewBooking("bk_1", "Anna", "prop_1", mustDate("2026-09-10"), mustDate("2026-09-14"))
	b.Confirm()
	if err := b.Confirm(); err != ErrCannotConfirm {
		t.Errorf("expected ErrCannotConfirm, got %v", err)
	}
}

func TestCancel_FromPending_Succeeds(t *testing.T) {
	b, _ := NewBooking("bk_1", "Anna", "prop_1", mustDate("2026-09-10"), mustDate("2026-09-14"))
	if err := b.Cancel(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Status != StatusCancelled {
		t.Errorf("expected cancelled, got %s", b.Status)
	}
}

func TestCancel_AlreadyCancelled_Fails(t *testing.T) {
	b, _ := NewBooking("bk_1", "Anna", "prop_1", mustDate("2026-09-10"), mustDate("2026-09-14"))
	b.Cancel()
	if err := b.Cancel(); err != ErrAlreadyCancelled && err != ErrCannotCancel {
		t.Errorf("expected a cancellation error, got %v", err)
	}
}

func TestCancel_AfterConfirm_Succeeds(t *testing.T) {
	b, _ := NewBooking("bk_1", "Anna", "prop_1", mustDate("2026-09-10"), mustDate("2026-09-14"))
	b.Confirm()
	if err := b.Cancel(); err != nil {
		t.Errorf("expected cancel to succeed from confirmed state, got %v", err)
	}
}
