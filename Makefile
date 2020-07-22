SHELL := bash

# Directory, where all required tools are located (absolute path required)
TOOLS_DIR ?= $(shell cd tools && pwd)

# Prerequisite tools
GO ?= go
DOCKER ?= docker

# Tools managed by this project
GINKGO ?= $(TOOLS_DIR)/ginkgo
LINTER ?= $(TOOLS_DIR)/golangci-lint
GOVERALLS ?= $(TOOLS_DIR)/goveralls
GOVER ?= $(TOOLS_DIR)/gover

.EXPORT_ALL_VARIABLES:
.PHONY: test lint fmt vet

test: fmt vet docker-is-running $(GINKGO)
	$(GINKGO) -r -v -cover pkg


test-%: fmt vet docker-is-running $(GINKGO)
	$(GINKGO) -r -v -cover pkg/$*

# First run gover to merge the coverprofiles and upload to coveralls
coverage: $(GOVERALLS) $(GOVER)
	$(GOVER)
	$(GOVERALLS) -coverprofile=gover.coverprofile -service=travis-ci -repotoken $(COVERALLS_TOKEN)

lint: $(LINTER) helm-lint
	$(GO) mod verify
	$(LINTER) run -v --no-config --deadline=5m

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

docker-is-running:
	@echo "Checking if docker is running..."
	@{ \
	set -e; \
	$(DOCKER) version > /dev/null; \
	}

tools: $(TOOLS_DIR)/ginkgo $(TOOLS_DIR)/golangci-lint $(TOOLS_DIR)/goveralls $(TOOLS_DIR)/gover

$(TOOLS_DIR)/ginkgo:
	$(shell $(TOOLS_DIR)/goget-wrapper github.com/onsi/ginkgo/ginkgo@v1.12.0)

$(TOOLS_DIR)/golangci-lint:
	$(shell curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(TOOLS_DIR) v1.25.0)

$(TOOLS_DIR)/goveralls:
	$(shell $(TOOLS_DIR)/goget-wrapper github.com/mattn/goveralls@v0.0.5)

$(TOOLS_DIR)/gover:
	$(shell $(TOOLS_DIR)/goget-wrapper github.com/modocache/gover)
