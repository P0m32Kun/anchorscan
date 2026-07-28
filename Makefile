APP := anchorscan
CMD := ./cmd/anchorscan
DIST_DIR := dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DISPLAY_VERSION := $(patsubst v%,%,$(VERSION))
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BINARY := $(APP)$(if $(filter windows,$(GOOS)),.exe,)
BUILD_FLAGS ?=
LDFLAGS ?=
VERSION_PACKAGE := github.com/P0m32Kun/anchorscan/internal/version
VERSION_LDFLAGS := $(strip $(LDFLAGS) -X $(VERSION_PACKAGE).Version=$(DISPLAY_VERSION))
PACKAGE_NAME := $(APP)-$(VERSION)-$(GOOS)-$(GOARCH)
PACKAGE_DIR := $(DIST_DIR)/$(PACKAGE_NAME)
PACKAGE_ARCHIVE ?= $(DIST_DIR)/$(PACKAGE_NAME).tar.gz
E2E_TIMEOUT ?= 55m
RUNTIME_CONFIG := default.yaml.example nse.yaml service-tags.yaml ports-highrisk.txt ports-top1000.txt

.PHONY: test doc-check docx-test docx-visual build package package-test package-smoke security-check web-smoke release-check pr-check e2e clean

test:
	go test ./...
	node --test internal/web/static/*.test.mjs internal/web/frontend/*.test.mjs

doc-check:
	node scripts/check_markdown_links.mjs

docx-test:
	uv run --project tools/docx-render python -m unittest discover -s tools/docx-render -p 'test_*.py'

docx-visual:
	uv run --project tools/docx-render python tools/docx-render/visual_check.py

web:
	npm run build:web

build: web
	mkdir -p $(DIST_DIR)
	go build $(BUILD_FLAGS) -ldflags="$(VERSION_LDFLAGS)" -o $(DIST_DIR)/$(BINARY) $(CMD)

package: web
	rm -rf $(PACKAGE_DIR)
	mkdir -p $(PACKAGE_DIR)/config $(PACKAGE_DIR)/docs $(PACKAGE_DIR)/tools/docx-render/templates
	go build $(BUILD_FLAGS) -ldflags="$(VERSION_LDFLAGS)" -o $(PACKAGE_DIR)/$(BINARY) $(CMD)
	cp $(addprefix config/,$(RUNTIME_CONFIG)) $(PACKAGE_DIR)/config/
	cp -R config/nuclei-templates $(PACKAGE_DIR)/config/
	@for file in $(RUNTIME_CONFIG); do \
		test -s "$(PACKAGE_DIR)/config/$$file" || { echo "missing required runtime config: $$file" >&2; exit 1; }; \
	done
	cp README.md $(PACKAGE_DIR)/docs/README.md
	cp docs/deploy.md $(PACKAGE_DIR)/docs/deploy.md
	cp tools/docx-render/.python-version tools/docx-render/pyproject.toml tools/docx-render/uv.lock tools/docx-render/render_docx.py $(PACKAGE_DIR)/tools/docx-render/
	cp tools/docx-render/templates/project-report.docx $(PACKAGE_DIR)/tools/docx-render/templates/project-report.docx
	tar -C $(DIST_DIR) -czf $(PACKAGE_ARCHIVE) $(PACKAGE_NAME)
	@for file in $(RUNTIME_CONFIG); do \
		tar -tzf "$(PACKAGE_ARCHIVE)" | grep -Fxq "$(PACKAGE_NAME)/config/$$file" || { echo "archive missing required runtime config: $$file" >&2; exit 1; }; \
	done

package-smoke:
	ANCHORSCAN_PACKAGE_ARCHIVE="$(abspath $(PACKAGE_ARCHIVE))" ANCHORSCAN_PACKAGE_NAME="$(PACKAGE_NAME)" ANCHORSCAN_PACKAGE_VERSION="$(DISPLAY_VERSION)" go test -tags packageintegration ./scripts

package-test: package package-smoke

security-check:
	go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
	npm audit --audit-level=high --registry=https://registry.npmjs.org
	uv lock --check --project tools/docx-render

web-smoke: build
	ANCHORSCAN_EXPECTED_VERSION=$(DISPLAY_VERSION) npm run test:web

release-check: web-smoke
	@test "$(GOOS)" = "$(shell go env GOOS)" && test "$(GOARCH)" = "$(shell go env GOARCH)" || { echo "release-check requires a host build" >&2; exit 1; }
	@test "$$($(DIST_DIR)/$(BINARY) version)" = "anchorscan version $(DISPLAY_VERSION)" || { echo "release version was not injected" >&2; exit 1; }

pr-check: test doc-check docx-test build package-test web-smoke

e2e:
	go test -tags=e2e ./e2e -timeout $(E2E_TIMEOUT) -v

clean:
	rm -rf $(DIST_DIR)
