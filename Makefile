-include .env
export

.PHONY: generate migrate migrate-down migrate-status run

generate:
	go tool oapi-codegen -generate types,chi-server -package api -o api/api.gen.go contracts/openapi/trip-service.openapi.yaml

migrate:
	goose -dir ./migrations postgres "$$DATABASE_URL" up

migrate-down:
	goose -dir ./migrations postgres "$$DATABASE_URL" down

migrate-status:
	goose -dir ./migrations postgres "$$DATABASE_URL" status

run:
	go run ./cmd/service $(ARGS)

test:
	go test -race -coverprofile=coverage.out -coverpkg=./internal/domain/...,./internal/usecase/... ./internal/domain/... ./internal/usecase/...
	go tool cover -func=coverage.out

test-integration:
	go test -tags=integration -v ./internal/repository/...

lint:
	golangci-lint run ./...