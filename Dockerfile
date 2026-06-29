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

# Final stage: distroless
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /app/semrel /semrel

ENTRYPOINT ["/semrel"]
