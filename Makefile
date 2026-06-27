.PHONY: run build clean check test tidy screenshots qa qa-list

run:
	EGL_LOG_LEVEL=fatal go run .

build:
	go build ./...

clean:
	go clean

check:
	go vet ./...

test:
	go test -short ./...

tidy:
	go mod tidy

screenshots:
	EGL_LOG_LEVEL=fatal go run ./cmd/screenshots

qa:
	go run ./cmd/qa -preset $(PRESET)

qa-list:
	go run ./cmd/qa -list
