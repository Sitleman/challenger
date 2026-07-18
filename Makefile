.PHONY: run test

run:
	@set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/challenger

test:
	go test ./...
