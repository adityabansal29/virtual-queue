.PHONY: up down logs verify test build simulate

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

simulate:
	python3 scripts/simulate-queue.py
