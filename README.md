# Go Transaction Service

A REST API built with **Go and PostgreSQL** to demonstrate reliable transaction processing: atomic balance updates, concurrency control, idempotent transfers, account-scoped access, and automated testing.

> **Portfolio / educational project only.** This service uses simulated accounts and money. It is **not** a production banking system and must not be used to process real funds.

## Features

- Atomic fund transfers: debit and credit succeed or roll back together.
- PostgreSQL row-level locking to protect balances from concurrent updates.
- Idempotency keys to prevent duplicate transfers; a reused key with different transfer details produces a conflict.
- Server-side account scoping for protected account and transfer endpoints.
- Transfer details and paginated history, including transfers sent and received by the authorized account.
- Request validation, consistent JSON errors, and `Cache-Control: no-store` on protected responses.
- Unit tests, PostgreSQL integration tests, and HTTP end-to-end tests using Go's race detector.
- Multi-stage Docker image with a non-root runtime; Docker Compose for local development.
- GitHub Actions workflow for tests, static analysis, and Docker builds.

## Tech stack

| Component | Technology |
| --- | --- |
| Language / HTTP | Go 1.25+; standard-library `net/http` |
| Database | PostgreSQL 17 |
| SQL driver | `github.com/jackc/pgx/v5` via `database/sql` |
| Tests | Go `testing`, `httptest`, race detector |
| Containers | Docker, Docker Compose, multi-stage Distroless runtime |
| CI definition | GitHub Actions (`.github/workflows/ci.yml`) |

The `go.mod` file declares Go 1.25.0. Docker builds and CI use Go 1.26.

## Architecture

```text
Client
  |
  v
HTTP router -> API key middleware -> HTTP handlers
                                        |
                     +------------------+-------------------+
                     |                                      |
              POST transfer                       Account / transfer reads
                     |                                      |
               TransferService                        Repositories
                     |                                      |
              TransferProcessor                             |
                     |                                      |
                     +---------------+----------------------+
                                     |
                                PostgreSQL
```

The write path runs a database transaction and locks accounts before changing balances. The read path applies the configured account scope in account and transfer queries. These protections are separate: **idempotency** prevents duplicate processing, while **authorization** controls which account can be used or viewed.

## API endpoints

| Method | Path | Purpose | Authentication |
| --- | --- | --- | --- |
| `GET` | `/healthz` | Health check | None |
| `GET` | `/v1/accounts/{id}` | Read the authorized account | `X-API-Key` |
| `POST` | `/v1/transfers` | Create or replay a transfer | `X-API-Key`, `Idempotency-Key` |
| `GET` | `/v1/transfers/{id}` | Read a transfer visible to the authorized account | `X-API-Key` |
| `GET` | `/v1/transfers?page=1&per_page=10` | List the authorized account's transfer history | `X-API-Key` |

History defaults to `page=1` and `per_page=10`; `per_page` is capped at **100**. History results are ordered newest first, with `id` as a tie-breaker. Unauthorized access to a transfer detail is returned as `404` to avoid disclosing whether a different account's transfer exists.

## OpenAPI Specification

The REST API is documented using OpenAPI 3.1.

See the [OpenAPI Specification](docs/openapi.yaml) for:

- Authentication and account-scoped authorization
- Request and response schemas
- Transfer idempotency behavior
- Transaction history pagination
- HTTP error responses

The specification can be opened with OpenAPI-compatible
documentation tools such as Swagger UI or Redoc.

### Create a transfer

Replace the example account IDs with IDs from your local database:

```bash
curl -i -X POST 'http://127.0.0.1:8080/v1/transfers' \
  -H 'X-API-Key: local-test-secret' \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-transfer-001' \
  -d '{"from_account_id":1,"to_account_id":2,"amount":1000}'
```

`amount` is a positive integer in the simulated IDR account balance units; floating-point values are not used. The sender must match the account configured by `API_ACCOUNT_ID`.

A new transfer returns **`201 Created`**. Sending the same request with the same idempotency key returns **`200 OK`** with `Idempotent-Replayed: true` and does not debit again. Reusing the key with different transfer details returns **`409 Conflict`**.

### Read transfer history

```bash
curl -i \
  -H 'X-API-Key: local-test-secret' \
  'http://127.0.0.1:8080/v1/transfers?page=1&per_page=10'
```

The response has the shape:

```json
{
  "data": [
    {
      "id": 1,
      "from_account_id": 1,
      "to_account_id": 2,
      "amount": 1000,
      "status": "completed"
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 10,
    "total": 1,
    "total_pages": 1
  }
}
```

IDs, balances, and history contents depend on your local test data. Validation errors and authorization failures are returned as JSON with an `error` field.

## Configuration

| Variable | Required | Description |
| --- | --- | --- |
| `DATABASE_URL` | Yes | PostgreSQL connection URL |
| `API_KEY` | Yes | Shared demonstration API key |
| `API_ACCOUNT_ID` | Yes | Positive numeric ID for the one authorized account |
| `LISTEN_ADDRESS` | No | Defaults to `127.0.0.1:8080`; Compose uses `0.0.0.0:8080` inside the container |
| `TEST_DATABASE_URL` | For integration/E2E tests | Must point to the separate `transactions_test` database |

Never commit actual secrets or a populated `.env` file. The credentials shown below are **local development examples only**.

## Local setup

**Prerequisites:** Go 1.25 or newer, Docker with Compose, and a terminal. The commands below assume the repository root.

### 1. Start PostgreSQL

```bash
docker compose up -d db
docker compose ps
```

The development database is created by the PostgreSQL container on its **first initialization** and is reachable from your host at `127.0.0.1:5436`. Docker Compose stores its data in a named volume.

### 2. Create the separate test database (one-time)

```bash
docker compose exec -T db \
  psql -X -v ON_ERROR_STOP=1 -U app -d postgres \
  -c 'CREATE DATABASE transactions_test;'
```

Skip this command if `transactions_test` already exists.

### 3. Apply schema migrations (one-time for each fresh database)

```bash
for database in transactions transactions_test; do
  for migration in migrations/*.sql; do
    echo "Applying $migration to $database"
    docker compose exec -T db \
      psql -X -v ON_ERROR_STOP=1 -U app -d "$database" \
      < "$migration" || exit 1
  done
done
```

**Important:** These SQL files are plain, ordered migrations—not an automatic, repeatable migration system. **Do not rerun them against an already migrated database**, or you may encounter duplicate-object errors. Neither `go run` nor Docker Compose applies them automatically.

### 4. Create example accounts (development database only)

```bash
docker compose exec -T db psql -X -v ON_ERROR_STOP=1 -U app -d transactions <<'SQL'
INSERT INTO accounts (account_number, currency, balance)
VALUES
  ('DEMO-SENDER-001', 'IDR', 500000),
  ('DEMO-RECIPIENT-001', 'IDR', 200000)
ON CONFLICT (account_number) DO NOTHING;

SELECT id, account_number, balance
FROM accounts
WHERE account_number IN ('DEMO-SENDER-001', 'DEMO-RECIPIENT-001')
ORDER BY id;
SQL
```

Note the returned IDs. Use the **actual sender ID** as `API_ACCOUNT_ID` and replace example transfer IDs in subsequent requests. Existing accounts may have different balances if you have already run transfers.

### 5. Run the API directly on your computer

```bash
export DATABASE_URL='postgres://app:localpassword@127.0.0.1:5436/transactions?sslmode=disable'
export API_KEY='local-test-secret'
export API_ACCOUNT_ID='1' # Replace 1 with your actual demo sender ID

go run ./cmd/api
```

The default listening address is `127.0.0.1:8080`. In a second terminal:

```bash
curl -i http://127.0.0.1:8080/healthz
```

## Run with Docker Compose

After setting up the schema and demo accounts as above, you can run the API in a container instead of running `go run`:

```bash
API_KEY='local-test-secret' API_ACCOUNT_ID='1' \
  docker compose up -d --build api
```

Replace `1` with your sender account ID. Compose connects the API to `db:5432`, binds the container to `0.0.0.0:8080`, and publishes it **only to your computer's loopback interface** at `127.0.0.1:8081`.

```bash
docker compose ps
docker compose logs --tail=30 api
curl -i http://127.0.0.1:8081/healthz
curl -i -H 'X-API-Key: local-test-secret' \
  'http://127.0.0.1:8081/v1/transfers?page=1&per_page=10'
```

The runtime image uses a non-root account (`65532:65532`). To stop services while **retaining** database data:

```bash
docker compose stop api
```

**Do not run `docker compose down -v` unless you deliberately want to delete the PostgreSQL data volume.**

## Tests and checks

Unit tests (no PostgreSQL required):

```bash
go test -count=1 ./...
```

Integration tests and HTTP end-to-end tests (requires the migrated `transactions_test` database and running PostgreSQL):

```bash
TEST_DATABASE_URL='postgres://app:localpassword@127.0.0.1:5436/transactions_test?sslmode=disable' \
  go test -race -tags=integration -count=1 ./...
```

Static analysis:

```bash
go vet -tags=integration ./...
```

Integration tests use isolated test fixtures and exercise real database transactions, balance changes, the HTTP router, idempotency replay, and conflict behavior. Tests using the `integration` build tag are **not** included in the default `go test ./...` run.

## Continuous integration

`.github/workflows/ci.yml` defines checks for pushes and pull requests to `main`: Go formatting, dependency verification, unit tests, a fresh PostgreSQL 17 service with ordered schema migrations, integration/E2E tests under the race detector, `go vet`, and a Docker build.

**Status:** These checks and the Docker build have passed locally. Execution of the workflow on GitHub-hosted runners has **not yet been verified**; CI will become observable after the repository and workflow are published to GitHub.

## Security model and limitations

Implemented safeguards include account-scoped access checks, parameterized SQL, transactional balance updates, idempotency constraints, input validation, protected-response cache headers, and a non-root container runtime.

This service is intentionally **not production-ready**:

- A single static API key and account ID stand in for identity and authorization. There is no multi-user authentication, key management, or OAuth2/OIDC.
- The balance-based transfer model is not a complete double-entry accounting ledger.
- There are no external banking integrations, real-money settlement, or regulatory controls.
- Deployment orchestration, production secret management, TLS termination, observability, and operational controls are outside the project's current scope.
- OFFSET-based history pagination is suitable for this demonstration, but changing data can cause page shifts across requests; cursor pagination would be preferable at large scale.

Run it only against **simulated development or test data**. Do not expose the demo API or development PostgreSQL credentials publicly.

## Project status

Core APIs, PostgreSQL transaction handling, automated tests, Docker, and the GitHub Actions workflow definition are implemented. Formal OpenAPI documentation and GitHub-hosted CI verification are planned follow-up milestones.
