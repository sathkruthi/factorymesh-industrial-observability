.PHONY: test check build integration up down demo setup incident recovery
test:
	go test -race ./...
check:
	go vet ./...
	test -z "$$(gofmt -l $$(find . -name '*.go'))"
build:
	go build ./...
integration:
	DATABASE_URL=postgres://factorymesh:local_demo_only@localhost:55432/factorymesh?sslmode=disable go test -race -tags=integration -count=1 ./internal/store ./internal/pipeline
up:
	docker compose up --build -d --wait
down:
	docker compose down
demo:
	python3 scripts/demo.py
setup:
	go mod download
incident:
	DATABASE_URL=postgres://factorymesh:local_demo_only@localhost:55432/factorymesh?sslmode=disable python3 scripts/incident.py $(ID)
recovery:
	python3 scripts/recovery.py
