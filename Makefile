# Variables
APP_NAME := ktool
BUILD_DIR := build
SRC_DIR := src
GO_FILES := $(shell find $(SRC_DIR) -name '*.go' -type f)
GO_LDFLAGS := -s -w
ARGS := ""
INSTALL_PATH := /usr/local/bin

VERSION := $(shell git describe --tags --always)
COMMIT := $(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

DOCKER_IMAGE= $(APP_NAME)-builder:$(VERSION)

# Build targets
.PHONY: all build docker-build clean run install install-docker

all: build

## Install binary in INSTALL_PATH
install: build
	@echo "Installing $(APP_NAME) in $(INSTALL_PATH)"
	sudo cp $(BUILD_DIR)/$(APP_NAME)  $(INSTALL_PATH)/
	@which $(APP_NAME)
	@echo "$(APP_NAME) successfully installed in $(INSTALL_PATH)"

## Install binary in INSTALL_PATH with docker-build
docker-install: docker-build
	@echo "Installing $(APP_NAME) in $(INSTALL_PATH)"
	sudo cp $(BUILD_DIR)/$(APP_NAME)  $(INSTALL_PATH)/
	@which $(APP_NAME)
	@echo "$(APP_NAME) successfully installed in $(INSTALL_PATH)"

## Build through Docker
docker-build: clean
	@echo "Building $(APP_NAME) through docker"
	@mkdir -p $(BUILD_DIR)
	docker build -t $(DOCKER_IMAGE) .
	docker run --rm \
		-v $(shell pwd)/$(BUILD_DIR):/$(BUILD_DIR) \
		$(DOCKER_IMAGE) cp /$(APP_NAME) /$(BUILD_DIR)/$(APP_NAME)
	docker image rm -f $(DOCKER_IMAGE)

## Build the Go binary
build: $(GO_FILES)
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build \
		-C $(SRC_DIR) \
		-ldflags="$(GO_LDFLAGS) -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)" \
		-o ../$(BUILD_DIR)/$(APP_NAME) .

## Run the application
run: build
	@echo "Running $(APP_NAME)..."
	./$(BUILD_DIR)/$(APP_NAME) $(ARGS)
run-go: 
	@echo "Running $(APP_NAME)..."
	@go run src/*.go $(ARGS)

## Clean the build directory
clean:
	@echo "Cleaning up..."
	rm -rf $(BUILD_DIR)

## Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy

## Display version information
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

## Help: list all make targets
help:
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'
