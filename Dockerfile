# Step 1: Modules caching
FROM golang:1.25-alpine3.21 AS modules

COPY go.mod go.sum /modules/

WORKDIR /modules

RUN go mod download

# Step 2: Builder
FROM golang:1.25-alpine3.21 AS builder

COPY --from=modules /go/pkg /go/pkg
COPY . /app

WORKDIR /app

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -tags migrate -o /bin/app ./cmd/app

# Step 3: Final
FROM scratch

# Copy CA certificates for HTTPS calls.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy passwd for non-root user.
COPY --from=builder /etc/passwd /etc/passwd

# Create non-root user in builder and copy.
COPY --from=builder /app/migrations /migrations
COPY --from=builder /bin/app /app

# Run as non-root user (nobody).
USER nobody

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app", "healthcheck"] || exit 1

CMD ["/app"]
