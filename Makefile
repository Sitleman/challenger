.PHONY: run test test-scripts set-status

run:
	@set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/challenger

test: test-scripts
	go test ./...

test-scripts:
	bash scripts/set-status_test.sh

set-status:
	@scripts/set-status.sh "$(ISSUE)" "$(STATUS)"
