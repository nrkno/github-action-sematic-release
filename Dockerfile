# Multi-stage build
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source
COPY . .

# Build arguments
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

# Build static binary (CGO_ENABLED=0)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o semrel \
    ./cmd/semrel

# Final stage: alpine (distroless/static has no shell; entrypoint.sh needs sh).
# nonroot user is created to avoid running as root.
FROM alpine:3.19

RUN addgroup -S nonroot && adduser -S nonroot -G nonroot

COPY --from=builder /app/semrel /semrel
COPY entrypoint.sh /entrypoint.sh

USER nonroot

ENTRYPOINT ["/entrypoint.sh"]
