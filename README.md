# Go Transaction Service

An educational REST API project for learning Go backend development and safe financial transaction processing. **This is a simulated portfolio project, not a production banking system.**

## Phase 1: HTTP service foundation

This first checkpoint uses **only Go standard-library packages**: no PostgreSQL, Docker, or third-party dependencies are required yet.

```bash
go version
go test ./...
go test -race ./...
go run ./cmd/api
```

In another terminal:

```bash
curl -i http://localhost:8080/healthz
```

Expected response: HTTP `200 OK` with `{"status":"ok"}`. The route accepts GET only; POST returns HTTP 405.

## Next phases

- Domain models and transfer validation
- PostgreSQL schema and data access
- Atomic transfers with locking and idempotency
- REST API for accounts and transfers
- Integration tests, Docker, CI and OpenAPI documentation

**Learning workflow:** Review each change, run tests, then create your own commit after verification. This starter contains no Git history; commit dates reflect your real progress.
