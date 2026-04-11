.PHONY: build test lint run docker-build docker-up docker-down migrate-up migrate-down

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# --- Build ---
build:
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o api ./cmd/api
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o worker ./cmd/worker

run: build
	./api

# --- Test ---
test:
	go test ./... -count=1 -timeout 120s

lint:
	golangci-lint run ./...

# --- Docker ---
docker-build:
	docker build --build-arg VERSION=$(VERSION) -t velo-api .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

# --- Migrations (manual, local dev) ---
migrate-up:
	migrate -database "$(DATABASE_URL)" -path migrations up

migrate-down:
	migrate -database "$(DATABASE_URL)" -path migrations down 1
