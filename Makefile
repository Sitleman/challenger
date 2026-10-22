.PHONY: run test sqlc

run:
	@set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/challenger

test:
	go test ./...

sqlc:
	sqlc generate -f internal/db/sqlc.yaml
