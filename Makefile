.PHONY: run test race vet build

run:
	go run ./cmd/orquestra

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	go build -trimpath -ldflags="-s -w" -o bin/orquestra ./cmd/orquestra