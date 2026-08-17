# booking-service

A small Go REST API demonstrating backend design choices relevant to a
travel/booking domain: clean API design for both human and AI-agent
clients, and DDD-flavored structure as a first step toward an
event-driven, service-oriented architecture.

This is a deliberately small, self-contained project — the goal is to make
the *design decisions* legible, not to build a large system.

## Running it

```bash
go run ./cmd/server
# in another terminal:
curl -X POST localhost:8080/v1/bookings \
  -d '{"guest_name":"Anna Novak","property_id":"prop_42","check_in":"2026-09-10","check_out":"2026-09-14"}'
```

Run tests:

```bash
go test ./...
```

## Architecture

```
cmd/server        -> wires everything together, starts the HTTP server
internal/domain    -> the Booking aggregate root, invariants, repository interface
internal/events     -> domain events + a swappable Publisher interface
internal/api        -> HTTP handlers, request/response DTOs, structured errors
openapi.yaml         -> versioned API contract
```

## Design choices, and why

**Aggregate root (`domain.Booking`).** All state transitions (`Confirm`,
`Cancel`) go through methods on `Booking` itself, not through fields set
directly by a handler. This is what keeps invariants — like "a cancelled
booking can't be cancelled again" — enforced in exactly one place instead of
re-implemented (and potentially forgotten) in every handler that touches a
booking.

**Repository interface, not a concrete store.** `internal/api` depends on
`domain.Repository`, an interface. The demo uses an in-memory implementation
for simplicity, but swapping in a Postgres-backed repository later is a
change contained entirely to `internal/domain` — the API layer doesn't
change at all. This is the same boundary-drawing instinct behind bounded
contexts in DDD: the domain model shouldn't know or care how it's persisted.

**Domain events, published through an interface.** `BookingCreated` and
`BookingCancelled` are published through an `events.Publisher` interface.
The demo's `InMemoryPublisher` just logs and keeps history, but the
interface is the same shape you'd use with a real broker (SQS, Kafka,
NATS). This is the first concrete step in transitioning a monolithic,
synchronous flow toward an event-driven architecture: other bounded
contexts (billing, notifications, a guest portal) could subscribe to these
events instead of the Booking service calling them directly and knowing
about their existence.

**Structured, machine-readable errors.** Every error response has a stable
`code` field (`invalid_date_range`, `booking_not_found`, `cannot_cancel`)
alongside a human `message`. A human client can show the message; an
agent/LLM-driven client can branch on `code` without parsing free text —
this matters specifically for API design aimed at automated/agentic
consumers, not just human developers.

**A dedicated `/summary` endpoint for agent consumption.** `GET
/v1/bookings/{id}/summary` returns pre-computed derived fields (`nights`,
a `human_summary` sentence) rather than making the caller compute date
arithmetic to reason about the booking. This is a small but deliberate
example of designing an endpoint specifically for how an LLM-driven client
consumes data, versus designing only for a human-facing UI.

**Versioned routes (`/v1/...`).** Behaviour can evolve without breaking
existing clients — human or automated — by introducing `/v2` alongside
`/v1` rather than changing behaviour under callers' feet.

## What's deliberately left out (for a real production version)

- Persistence: swap `InMemoryRepository` for a Postgres-backed one behind
  the same `domain.Repository` interface.
- Event delivery: swap `InMemoryPublisher` for a real broker client behind
  the same `events.Publisher` interface.
- AuthN/AuthZ, rate limiting, request validation middleware.
- Idempotency keys on `POST /v1/bookings` for safe client retries.
