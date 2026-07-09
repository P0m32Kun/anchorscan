APP := anchorscan
CMD := ./cmd/anchorscan
DIST_DIR := dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
PACKAGE_NAME := $(APP)-$(VERSION)-$(GOOS)-$(GOARCH)
PACKAGE_DIR := $(DIST_DIR)/$(PACKAGE_NAME)

.PHONY: test build package clean

test:
	go test ./...
	node --test internal/web/static/app.test.mjs

build:
	mkdir -p $(DIST_DIR)
	go build -o $(DIST_DIR)/$(APP) $(CMD)

package:
	rm -rf $(PACKAGE_DIR)
	mkdir -p $(PACKAGE_DIR)/config $(PACKAGE_DIR)/docs
	go build -o $(PACKAGE_DIR)/$(APP) $(CMD)
	cp config/default.yaml.example $(PACKAGE_DIR)/config/default.yaml.example
	cp README.md $(PACKAGE_DIR)/docs/README.md
	cp docs/deploy.md $(PACKAGE_DIR)/docs/deploy.md
	tar -C $(DIST_DIR) -czf $(DIST_DIR)/$(PACKAGE_NAME).tar.gz $(PACKAGE_NAME)

clean:
	rm -rf $(DIST_DIR)
