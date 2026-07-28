APP := anchorscan
CMD := ./cmd/anchorscan
DIST_DIR := dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BINARY := $(APP)$(if $(filter windows,$(GOOS)),.exe,)
BUILD_FLAGS ?=
LDFLAGS ?=
VERSION_PACKAGE := github.com/P0m32Kun/anchorscan/internal/version
VERSION_LDFLAGS = -X $(VERSION_PACKAGE).Version=$(VERSION) $(LDFLAGS)
PACKAGE_NAME := $(APP)-$(VERSION)-$(GOOS)-$(GOARCH)
PACKAGE_DIR := $(DIST_DIR)/$(PACKAGE_NAME)
PACKAGE_ARCHIVE ?= $(DIST_DIR)/$(PACKAGE_NAME).tar.gz

.PHONY: test docx-test build package package-test package-smoke security-check web-smoke pr-check clean

test:
	go test ./...
	node --test internal/web/static/*.test.mjs internal/web/frontend/*.test.mjs

docx-test:
	uv run --project tools/docx-render python -m unittest discover -s tools/docx-render -p 'test_*.py'

web:
	npm run build:web

build: web
	mkdir -p $(DIST_DIR)
	go build $(BUILD_FLAGS) -ldflags="$(VERSION_LDFLAGS)" -o $(DIST_DIR)/$(BINARY) $(CMD)

package: web
	rm -rf $(PACKAGE_DIR)
	mkdir -p $(PACKAGE_DIR)/config $(PACKAGE_DIR)/docs $(PACKAGE_DIR)/tools/docx-render/templates
	go build $(BUILD_FLAGS) -ldflags="$(VERSION_LDFLAGS)" -o $(PACKAGE_DIR)/$(BINARY) $(CMD)
	cp config/default.yaml.example config/nse.yaml config/service-tags.yaml config/ports-highrisk.txt config/ports-top1000.txt $(PACKAGE_DIR)/config/
	cp -R config/nuclei-templates $(PACKAGE_DIR)/config/
	cp README.md $(PACKAGE_DIR)/docs/README.md
	cp docs/deploy.md $(PACKAGE_DIR)/docs/deploy.md
	cp tools/docx-render/.python-version tools/docx-render/pyproject.toml tools/docx-render/uv.lock tools/docx-render/render_docx.py $(PACKAGE_DIR)/tools/docx-render/
	cp tools/docx-render/templates/project-report.docx $(PACKAGE_DIR)/tools/docx-render/templates/project-report.docx
	tar -C $(DIST_DIR) -czf $(PACKAGE_ARCHIVE) $(PACKAGE_NAME)

package-smoke:
	ANCHORSCAN_PACKAGE_ARCHIVE="$(abspath $(PACKAGE_ARCHIVE))" ANCHORSCAN_PACKAGE_NAME="$(PACKAGE_NAME)" ANCHORSCAN_PACKAGE_VERSION="$(VERSION)" go test -tags packageintegration ./scripts

package-test: package package-smoke

security-check:
	go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
	npm audit --audit-level=high --registry=https://registry.npmjs.org
	uv lock --check --project tools/docx-render

web-smoke: build
	npm run test:web

pr-check: test docx-test build package-test web-smoke

clean:
	rm -rf $(DIST_DIR)
