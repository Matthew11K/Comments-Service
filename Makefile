GO ?= go
APP ?= comment-service
POSTGRES_DSN ?= postgres://comment:comment@localhost:5433/comment_service?sslmode=disable
COVERAGE_FILE ?= coverage.out
COVERAGE_TMP_FILE ?= $(COVERAGE_FILE).tmp
COVERAGE_PKG ?= ./...

.PHONY: generate mocks test lint migrate-up migrate-down docker-up docker-down

generate:
	$(GO) generate ./...

mocks:
	$(GO) run github.com/vektra/mockery/v3@v3.7.0

test:
	@TZ=UTC $(GO) test -coverpkg='$(COVERAGE_PKG)' --race -count=1 -coverprofile='$(COVERAGE_TMP_FILE)' ./...
	@grep -v '/mocks/' '$(COVERAGE_TMP_FILE)' > '$(COVERAGE_FILE)'
	@rm -f '$(COVERAGE_TMP_FILE)'
	@$(GO) tool cover -func='$(COVERAGE_FILE)' | grep '^total' | tr -s '\t'

lint:
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.1 run ./...

migrate-up:
	$(GO) run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database "$(POSTGRES_DSN)" up

migrate-down:
	$(GO) run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0 -path migrations -database "$(POSTGRES_DSN)" down 1

docker-up:
	docker compose up -d postgres

docker-down:
	docker compose down
