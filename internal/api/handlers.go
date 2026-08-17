// Package api exposes the Booking domain over HTTP.
//
// Design choices aimed at both human and agent/LLM clients:
//   - Every error response has a stable machine-readable `code` field in
//     addition to a human `message` — an agent parsing a failure can branch
//     on `code` without needing to pattern-match free text.
//   - Endpoints are versioned under /v1 so behaviour can evolve without
//     breaking existing clients (human or automated).
//   - A dedicated GET /v1/bookings/{id}/summary endpoint returns a flat,
//     pre-formatted natural-language-friendly summary alongside structured
//     fields — useful for an LLM-driven client that wants to reason about
//     a booking without itself computing derived fields like night count
//     or human-readable status.
package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rosariocolletti/booking-service/internal/domain"
	"github.com/rosariocolletti/booking-service/internal/events"
)

// newID generates a short random hex identifier. A production service
// would likely use github.com/google/uuid or a ULID library instead; this
// keeps the demo dependency-free.
func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("bk_%x", b)
}

type Server struct {
	repo      domain.Repository
	publisher events.Publisher
	mux       *http.ServeMux
}

func NewServer(repo domain.Repository, publisher events.Publisher) *Server {
	s := &Server{repo: repo, publisher: publisher, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /v1/bookings", s.createBooking)
	s.mux.HandleFunc("GET /v1/bookings", s.listBookings)
	s.mux.HandleFunc("GET /v1/bookings/{id}", s.getBooking)
	s.mux.HandleFunc("GET /v1/bookings/{id}/summary", s.getBookingSummary)
	s.mux.HandleFunc("POST /v1/bookings/{id}/confirm", s.confirmBooking)
	s.mux.HandleFunc("POST /v1/bookings/{id}/cancel", s.cancelBooking)
	s.mux.HandleFunc("GET /openapi.yaml", s.serveSpecFile)
	s.mux.HandleFunc("GET /healthz", s.healthz)
}

// --- structured error envelope -------------------------------------------

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: apiError{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// --- request/response DTOs -------------------------------------------------

type createBookingRequest struct {
	GuestName  string `json:"guest_name"`
	PropertyID string `json:"property_id"`
	CheckIn    string `json:"check_in"`  // RFC3339 date, e.g. "2026-09-10"
	CheckOut   string `json:"check_out"` // RFC3339 date
}

type bookingResponse struct {
	ID         string `json:"id"`
	GuestName  string `json:"guest_name"`
	PropertyID string `json:"property_id"`
	CheckIn    string `json:"check_in"`
	CheckOut   string `json:"check_out"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func toResponse(b *domain.Booking) bookingResponse {
	return bookingResponse{
		ID:         b.ID,
		GuestName:  b.GuestName,
		PropertyID: b.PropertyID,
		CheckIn:    b.CheckIn.Format("2006-01-02"),
		CheckOut:   b.CheckOut.Format("2006-01-02"),
		Status:     string(b.Status),
		CreatedAt:  b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  b.UpdatedAt.Format(time.RFC3339),
	}
}

// bookingSummaryResponse is deliberately flatter and includes derived,
// pre-computed fields (nights, human_summary) so an LLM-driven client
// doesn't need to compute date arithmetic itself just to reason about
// the booking in a response.
type bookingSummaryResponse struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	Nights        int    `json:"nights"`
	HumanSummary  string `json:"human_summary"`
}

// --- handlers ----------------------------------------------------------

// serveSpecFile serves the raw OpenAPI spec so tools (Postman, codegen,
// agent frameworks) can fetch a stable, machine-readable contract for this
// API at a predictable URL — a common convention alongside human-facing
// docs UIs like /docs or /swagger-ui (not implemented here).
func (s *Server) serveSpecFile(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("openapi.yaml")
	if err != nil {
		writeError(w, http.StatusNotFound, "spec_not_found", "openapi.yaml not found")
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Server) createBooking(w http.ResponseWriter, r *http.Request) {
	var req createBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}

	checkIn, err := time.Parse("2006-01-02", req.CheckIn)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_check_in", "check_in must be YYYY-MM-DD")
		return
	}
	checkOut, err := time.Parse("2006-01-02", req.CheckOut)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_check_out", "check_out must be YYYY-MM-DD")
		return
	}
	if req.GuestName == "" || req.PropertyID == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "guest_name and property_id are required")
		return
	}

	booking, err := domain.NewBooking(newID(), req.GuestName, req.PropertyID, checkIn, checkOut)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidDateRange) {
			writeError(w, http.StatusUnprocessableEntity, "invalid_date_range", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create booking")
		return
	}

	if err := s.repo.Save(booking); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not persist booking")
		return
	}

	s.publisher.Publish(events.Event{
		Name: "BookingCreated",
		Payload: events.BookingCreatedPayload{
			BookingID:  booking.ID,
			PropertyID: booking.PropertyID,
		},
	})

	writeJSON(w, http.StatusCreated, toResponse(booking))
}

func (s *Server) getBooking(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	booking, err := s.repo.FindByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "booking_not_found", fmt.Sprintf("no booking with id %q", id))
		return
	}
	writeJSON(w, http.StatusOK, toResponse(booking))
}

func (s *Server) getBookingSummary(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	booking, err := s.repo.FindByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "booking_not_found", fmt.Sprintf("no booking with id %q", id))
		return
	}
	nights := int(booking.CheckOut.Sub(booking.CheckIn).Hours() / 24)
	summary := fmt.Sprintf("%s has a %s booking for %d night(s) at property %s, from %s to %s.",
		booking.GuestName, booking.Status, nights, booking.PropertyID,
		booking.CheckIn.Format("Jan 2, 2006"), booking.CheckOut.Format("Jan 2, 2006"))

	writeJSON(w, http.StatusOK, bookingSummaryResponse{
		ID:           booking.ID,
		Status:       string(booking.Status),
		Nights:       nights,
		HumanSummary: summary,
	})
}

func (s *Server) listBookings(w http.ResponseWriter, r *http.Request) {
	bookings, err := s.repo.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not list bookings")
		return
	}
	out := make([]bookingResponse, 0, len(bookings))
	for _, b := range bookings {
		out = append(out, toResponse(b))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) confirmBooking(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	booking, err := s.repo.FindByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "booking_not_found", fmt.Sprintf("no booking with id %q", id))
		return
	}
	if err := booking.Confirm(); err != nil {
		writeError(w, http.StatusConflict, "cannot_confirm", err.Error())
		return
	}
	s.repo.Save(booking)
	writeJSON(w, http.StatusOK, toResponse(booking))
}

func (s *Server) cancelBooking(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	booking, err := s.repo.FindByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "booking_not_found", fmt.Sprintf("no booking with id %q", id))
		return
	}
	if err := booking.Cancel(); err != nil {
		writeError(w, http.StatusConflict, "cannot_cancel", err.Error())
		return
	}
	s.repo.Save(booking)

	s.publisher.Publish(events.Event{
		Name:    "BookingCancelled",
		Payload: events.BookingCancelledPayload{BookingID: booking.ID},
	})

	writeJSON(w, http.StatusOK, toResponse(booking))
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
