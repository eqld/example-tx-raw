.PHONY: run clean up down logs

DB_CONTAINER_NAME=postgres_tx_raw_example

# Default Go executable
GO=go

# Run the example: starts db, runs go app, then cleans up
run: up
	@echo "\nRunning Go application..."
	$(GO) run main.go
	@echo "\nApplication finished."
	@make down SILENT_DOWN=true

# Start and initialize the PostgreSQL container
up:
	@echo "Starting PostgreSQL container ($(DB_CONTAINER_NAME))..."
	docker-compose up -d postgres-example
	@echo "Waiting for PostgreSQL to be ready..."
	@until docker-compose exec postgres-example pg_isready -U exampleuser -d exampledb -q; do \
		printf "."; \
		sleep 1; \
	done
	@echo "\nPostgreSQL is ready."

# Stop and remove the PostgreSQL container
down:
ifeq ($(SILENT_DOWN),true)
	@echo "Stopping and removing PostgreSQL container ($(DB_CONTAINER_NAME))..."
	@docker-compose down -v --remove-orphans > /dev/null 2>&1
else
	@echo "Stopping and removing PostgreSQL container ($(DB_CONTAINER_NAME))..."
	docker-compose down -v --remove-orphans
endif

# Clean all Docker resources (useful for a fresh start)
clean: down
	@echo "Cleaning up Docker resources (images, volumes)..."
	# Add any specific image/volume cleanup if necessary, e.g.:
	# docker rmi postgres:15-alpine || true
	# docker volume rm $$(docker volume ls -qf dangling=true) || true
	@echo "Cleanup complete."

# View logs of the PostgreSQL container
logs:
	docker-compose logs -f postgres-example

# Simple build command (optional, as 'go run' also compiles)
build:
	@echo "Building Go application..."
	$(GO) build -o tx_raw_example main.go
	@echo "Build complete: ./tx_raw_example"

# Target to initialize Go module
init-mod:
	$(GO) mod init github.com/eqld/example-tx-raw
	$(GO) mod tidy
