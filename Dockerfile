# ---- Build stage ----
FROM golang:1.22-alpine AS builder

# Install build deps (just in case)
RUN apk add --no-cache git

WORKDIR /app

# Cache modules first
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build the app
# - CGO disabled for simpler static-ish binary
# - Trim path + strip symbols for smaller size
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -o actionlogger -ldflags="-s -w" ./...

# ---- Runtime stage ----
FROM alpine:3.20

# Install CA certs for HTTPS (Loki, etc.)
RUN apk add --no-cache ca-certificates && \
    update-ca-certificates

WORKDIR /app

# Create non-root user
RUN adduser -D -g '' appuser

# Copy binary from builder
COPY --from=builder /app/actionlogger /app/actionlogger

# Create data dir and ensure permissions
RUN mkdir -p /app/data && chown -R appuser:appuser /app

USER appuser

EXPOSE 3000

# Healthcheck is optional, but handy if you add /health later
# HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:3000/health || exit 1

CMD ["./actionlogger"]