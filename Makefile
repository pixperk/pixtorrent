build:
	@go build -o bin/pixtorrent


build-tracker:
	@go build -o bin/tracker ./cmd/tracker


dkr:
	@docker compose up -d

run-tracker: dkr build-tracker
	@./bin/tracker



run: build
	@./bin/pixtorrent




test:
	@go test ./...
