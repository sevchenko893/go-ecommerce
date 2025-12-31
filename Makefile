.PHONY: run test load-test bench clean

# Go parameters
BINARY_NAME=go-ecommerce
MAIN_PATH=cmd/web/main.go

all: build

build:
	@echo "Building binary..."
	go build -o $(BINARY_NAME) $(MAIN_PATH)

run:
	@echo "Starting server..."
	go run $(MAIN_PATH)

dev:
	@echo "Starting server in development mode..."
	ENV=development go run $(MAIN_PATH)

test:
	@echo "Running unit tests..."
	go test ./... -v

bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

load-test:
	@echo "Running load tests..."
	@if [ ! -f tests/load/vegeta_test.sh ]; then \
		echo "Downloading vegeta..."; \
		go install github.com/tsenart/vegeta@latest; \
	fi
	@chmod +x tests/load/vegeta_test.sh
	@./tests/load/vegeta_test.sh

install-tools:
	@echo "Installing required tools..."
	go install github.com/tsenart/vegeta@latest
	go install github.com/rakyll/hey@latest

check-race:
	@echo "Checking for race conditions..."
	go test -race ./...

profile:
	@echo "Generating CPU profile..."
	go test -bench=. -cpuprofile=cpu.prof
	go tool pprof -http=:8081 cpu.prof

clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
	rm -f *.prof
	rm -f *.bin
	rm -f report.txt

docker-build:
	@echo "Building Docker image..."
	docker build -t go-ecommerce .

docker-run:
	@echo "Running in Docker..."
	docker run -p 8080:8080 go-ecommerce

help:
	@echo "Available commands:"
	@echo "  make run        - Start the server"
	@echo "  make dev        - Start in development mode"
	@echo "  make build      - Build binary"
	@echo "  make test       - Run unit tests"
	@echo "  make load-test  - Run load tests with Vegeta"
	@echo "  make bench      - Run benchmarks"
	@echo "  make check-race - Check for race conditions"
	@echo "  make profile    - Generate CPU profile"
	@echo "  make clean      - Clean build artifacts"