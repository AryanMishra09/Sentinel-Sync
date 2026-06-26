# Stage 1 — compile
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Copy go.mod/go.sum first so the dependency layer is cached and only
# re-downloads when dependencies actually change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o bin/replica ./cmd/replica

# Stage 2 — run
FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/bin/replica .
EXPOSE 8080
CMD ["./replica"]
