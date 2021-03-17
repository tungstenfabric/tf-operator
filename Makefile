#!/usr/bin/env bash

SHELL:=/usr/bin/env bash

export GO111MODULE=on

# Tools.
TOOLS_DIR := contrib/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
GOLANGCI_LINT := $(abspath $(TOOLS_BIN_DIR)/golangci-lint)
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/controller-gen

.PHONY: test
test:
	$(TOOLS_DIR)/test.sh

$(GOLANGCI_LINT): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && go build -tags=tools -o bin/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint

$(CONTROLLER_GEN): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && go build -tags=tools -o bin/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

.PHONY: lint
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run -v

.PHONY: modules
modules:
	go mod tidy
	cd $(TOOLS_DIR); go mod tidy

.PHONY: generate
generate: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) object paths="./pkg/apis/tf/v1alpha1/..."

.PHONY: clean
clean:
	rm -rf $(TOOLS_BIN_DIR)/bin

.PHONY: verify-modules
verify-modules: modules
	@if !(git diff --quiet HEAD -- go.sum go.mod); then \
		echo "go module files are out of date, please run 'make modules'"; exit 1; \
	fi
