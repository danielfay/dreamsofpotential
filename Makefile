.PHONY: run build clean check test tidy screenshots

run:
	EGL_LOG_LEVEL=fatal go run .

build:
	go build ./...

clean:
	go clean

check:
	go vet ./...

test:
	go test ./...

tidy:
	go mod tidy

screenshots:
	EGL_LOG_LEVEL=fatal go run ./cmd/screenshots
