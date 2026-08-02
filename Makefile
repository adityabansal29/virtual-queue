.PHONY: up down logs verify test build

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

verify:
	./scripts/verify.sh

test:
	go test ./...

build:
	go build ./...
