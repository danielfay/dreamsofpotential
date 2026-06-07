.PHONY: run build clean check test tidy

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
