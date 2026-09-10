.PHONY: up down build logs migrate seed test

build:
	docker compose build

up:
	docker compose up -d postgres migrate app

down:
	docker compose down

logs:
	docker compose logs -f

migrate:
	docker compose run --rm migrate

seed:
	docker compose --profile seed run --rm seed

test:
	go test ./...
