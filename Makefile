build:
	@go build -o bin/pixtorrent

run : build
	@./bin/pixtorrent

test:
	@go test ./...