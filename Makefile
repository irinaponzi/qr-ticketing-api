.PHONY: build run-ticket run-validator run-qr-worker infra infra-down test test-cover tidy lint wiki

# Load .env if it exists (variables are prefixed per service).
-include .env

# Use public Go proxy instead of corporate registry for this personal project.
export GOPROXY=https://proxy.golang.org,direct
export GONOSUMCHECK=*

tidy:
	go mod tidy

build: tidy
	go build -o bin/ticket-api ./cmd/ticket-api
	go build -o bin/validator-api ./cmd/validator-api
	go build -o bin/qr-worker ./cmd/qr-worker

run-ticket:
	@PORT=$(TICKET_PORT) \
	DB_HOST=$(TICKET_DB_HOST) \
	DB_PORT=$(TICKET_DB_PORT) \
	DB_USER=$(TICKET_DB_USER) \
	DB_PASSWORD=$(TICKET_DB_PASSWORD) \
	DB_NAME=$(TICKET_DB_NAME) \
	RABBITMQ_URL=$(RABBITMQ_URL) \
	go run ./cmd/ticket-api

run-validator:
	@PORT=$(VALIDATOR_PORT) \
	REDIS_HOST=$(VALIDATOR_REDIS_HOST) \
	REDIS_PORT=$(REDIS_PORT) \
	RABBITMQ_URL=$(RABBITMQ_URL) \
	TICKET_SERVICE_URL=$(VALIDATOR_TICKET_SERVICE_URL) \
	HMAC_SECRET=$(HMAC_SECRET) \
	go run ./cmd/validator-api

run-qr-worker:
	@RABBITMQ_URL="$(RABBITMQ_URL)" \
	SMTP_HOST="$(SMTP_HOST)" \
	SMTP_PORT="$(SMTP_PORT)" \
	SMTP_FROM="$(SMTP_FROM)" \
	SMTP_USER="$(SMTP_USER)" \
	SMTP_PASSWORD="$(SMTP_PASSWORD)" \
	QR_SIZE="$(QR_SIZE)" \
	HMAC_SECRET="$(HMAC_SECRET)" \
	go run ./cmd/qr-worker

infra:
	docker compose up -d

infra-down:
	docker compose down

test:
	go test ./... -v -count=1

test-cover:
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -func=coverage.out
	@echo ""
	@echo "HTML report: go tool cover -html=coverage.out"

lint:
	@which golangci-lint > /dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; }
	golangci-lint run ./...

wiki:
	docker compose --profile docs up -d wiki
	@echo "Wiki available at http://localhost:8888"
