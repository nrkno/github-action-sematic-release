# Multi-stage build
FROM golang:1.25-alpine@sha256:523c3effe300580ed375e43f43b1c9b091b68e935a7c3a92bfcc4e7ed55b18c2 AS builder

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
# Must run as root: GitHub Actions mounts the workspace owned by the runner
# user (root), so a non-root USER cannot write to .git/ or $GITHUB_OUTPUT.
FROM alpine:3.19@sha256:6baf43584bcb78f2e5847d1de515f23499913ac9f12bdf834811a3145eb11ca1

COPY --from=builder /app/semrel /semrel
COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
