.PHONY: all build clean test lint docker docker-push

GOLANG_VERSION ?= 1.26
BINARY_NAME ?= x-ui
BUILD_FLAGS ?= -ldflags "-w -s -trimpath"

all: build

build:
	@echo "Building 3x-ui..."
	@mkdir -p build
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o build/$(BINARY_NAME)-linux-amd64 main.go
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o build/$(BINARY_NAME)-linux-arm64 main.go
	CGO_ENABLED=1 GOOS=linux GOARCH=arm go build $(BUILD_FLAGS) -o build/$(BINARY_NAME)-linux-arm main.go
	@echo "Build complete!"

build-all:
	@echo "Building for all platforms..."
	@mkdir -p build
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o build/$(BINARY_NAME)-linux-amd64 main.go
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o build/$(BINARY_NAME)-linux-arm64 main.go
	CGO_ENABLED=1 GOOS=linux GOARCH=arm go build $(BUILD_FLAGS) -o build/$(BINARY_NAME)-linux-arm main.go
	CGO_ENABLED=1 GOOS=linux GOARCH=386 go build $(BUILD_FLAGS) -o build/$(BINARY_NAME)-linux-386 main.go
	@echo "All builds complete!"

clean:
	@echo "Cleaning..."
	rm -rf build/*
	@echo "Clean complete!"

test:
	@echo "Running tests..."
	go test -v ./...

lint:
	@echo "Running linter..."
	golangci-lint run --timeout 5m

frontend:
	@echo "Building frontend..."
	cd frontend && npm run build

docker:
	@echo "Building Docker image..."
	docker build -t anishtayin/3x-ui:latest .

docker-push:
	@echo "Pushing Docker image..."
	docker push anishtayin/3x-ui:latest

release:
	@echo "Creating release with goreleaser..."
	goreleaser release --clean

.PHONY: docker-buildx
docker-buildx:
	@echo "Building multi-arch Docker image..."
	docker buildx build --platform linux/amd64,linux/arm64,linux/arm \
		--output "type=image,push=true" \
		--tag anishtayin/3x-ui:latest \
		--tag anishtayin/3x-ui:$(shell git rev-parse --short HEAD) \
		.
