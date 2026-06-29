.PHONY: build test test-race lint integration clean

build:
	CGO_ENABLED=0 go build -o bin/semrel ./cmd/semrel

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run

integration:
	go test -tags integration -race ./internal/cli/...

clean:
	rm -rf bin/ *.test coverage.out
