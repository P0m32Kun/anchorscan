APP := anchorscan
CMD := ./cmd/anchorscan
DIST_DIR := dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BINARY := $(APP)$(if $(filter windows,$(GOOS)),.exe,)
BUILD_FLAGS ?=
LDFLAGS ?=
PACKAGE_NAME := $(APP)-$(VERSION)-$(GOOS)-$(GOARCH)
PACKAGE_DIR := $(DIST_DIR)/$(PACKAGE_NAME)

.PHONY: test docx-test build package web-smoke pr-check clean

test:
	go test ./...
	node --test internal/web/static/app.test.mjs

docx-test:
	uv run --project tools/docx-render python -m unittest discover -s tools/docx-render -p 'test_*.py'

build:
	mkdir -p $(DIST_DIR)
	go build $(BUILD_FLAGS) $(if $(LDFLAGS),-ldflags="$(LDFLAGS)") -o $(DIST_DIR)/$(BINARY) $(CMD)

package:
	rm -rf $(PACKAGE_DIR)
	mkdir -p $(PACKAGE_DIR)/config $(PACKAGE_DIR)/docs $(PACKAGE_DIR)/tools/docx-render/templates
	go build $(BUILD_FLAGS) $(if $(LDFLAGS),-ldflags="$(LDFLAGS)") -o $(PACKAGE_DIR)/$(BINARY) $(CMD)
	cp config/default.yaml.example $(PACKAGE_DIR)/config/default.yaml.example
	cp README.md $(PACKAGE_DIR)/docs/README.md
	cp docs/deploy.md $(PACKAGE_DIR)/docs/deploy.md
	cp tools/docx-render/.python-version tools/docx-render/pyproject.toml tools/docx-render/uv.lock tools/docx-render/render_docx.py $(PACKAGE_DIR)/tools/docx-render/
	cp tools/docx-render/templates/project-report.docx $(PACKAGE_DIR)/tools/docx-render/templates/project-report.docx
	tar -C $(DIST_DIR) -czf $(DIST_DIR)/$(PACKAGE_NAME).tar.gz $(PACKAGE_NAME)

web-smoke: build
	npm run test:web

pr-check: test docx-test build package web-smoke

clean:
	rm -rf $(DIST_DIR)
