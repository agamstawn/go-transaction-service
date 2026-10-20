# Stage 1: Build the Go application.
FROM golang:1.26-alpine3.24 AS builder

WORKDIR /src

# Download dependencies separately for Docker layer caching.
COPY go.mod go.sum ./

RUN go mod download

# Copy application source.
COPY . .

# Build a statically linked Linux binary.
RUN CGO_ENABLED=0 GOOS=linux \
    go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/api \
    ./cmd/api


# Stage 2: Minimal non-root runtime.
FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /

COPY --from=builder /out/api /api

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/api"]
