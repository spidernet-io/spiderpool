#!/usr/bin/make -f

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0


include ./Makefile.defs

all: build-bin install-bin

.PHONY: all build install

SUBDIRS := cmd/spiderpool-agent cmd/spiderpool-controller cmd/spiderpoolctl cmd/spiderpool


build-bin:
	for i in $(SUBDIRS); do $(MAKE) $(SUBMAKEOPTS) -C $$i all; done

install-bin:
	$(QUIET)$(INSTALL) -m 0755 -d $(DESTDIR_BIN)
	for i in $(SUBDIRS); do $(MAKE) $(SUBMAKEOPTS) -C $$i install; done

install-bash-completion:
	$(QUIET)$(INSTALL) -m 0755 -d $(DESTDIR_BIN)
	for i in $(SUBDIRS); do $(MAKE) $(SUBMAKEOPTS) -C $$i install-bash-completion; done

clean:
	-$(QUIET) for i in $(SUBDIRS); do $(MAKE) $(SUBMAKEOPTS) -C $$i clean; done
	-$(QUIET) rm -rf $(DESTDIR_BIN)
	-$(QUIET) rm -rf $(DESTDIR_BASH_COMPLETION)

.PHONY: lint
lint-golang:
	@$(ECHO_CHECK) contrib/scripts/check-go-fmt.sh
	$(QUIET) contrib/scripts/check-go-fmt.sh
	@$(ECHO_CHECK) contrib/scripts/lock-check.sh
	$(QUIET) contrib/scripts/lock-check.sh
	@$(ECHO_CHECK) vetting all GOFILES...
	$(QUIET) $(GO_VET) \
    ./cmd/... \
    ./pkg/... \
    ./test/...  \
    ./contrib/...
	@$(ECHO_CHECK) golangci-lint
	$(QUIET) golangci-lint run

.PHONY: lint-markdown
lint-markdown:
	@$(CONTAINER_ENGINE) container run --rm \
		--entrypoint sh -v $(ROOT_DIR):/workdir ghcr.io/igorshubovych/markdownlint-cli:latest \
		-c '/usr/local/bin/markdownlint -c /workdir/.github/markdownlint.yaml -p /workdir/.github/markdownlintignore  /workdir/'

.PHONY: fix-markdown
fix-markdown:
	@$(CONTAINER_ENGINE) container run --rm  \
		--entrypoint sh -v $(ROOT_DIR):/workdir ghcr.io/igorshubovych/markdownlint-cli:latest \
		-c '/usr/local/bin/markdownlint -f -c /workdir/.github/markdownlint.yaml -p /workdir/.github/markdownlintignore  /workdir/'

.PHONY: lint-yaml
lint-yaml:
	@$(CONTAINER_ENGINE) container run --rm \
		--entrypoint sh -v $(ROOT_DIR):/data cytopia/yamllint \
		-c '/usr/bin/yamllint -c /data/.github/yamllint-conf.yml /data'


.PHONY: lint-openapi
lint-openapi:
	@$(CONTAINER_ENGINE) container run --rm \
		-v $(ROOT_DIR):/spec redocly/openapi-cli lint api/v1/agent/openapi.yaml
	@$(CONTAINER_ENGINE) container run --rm \
		-v $(ROOT_DIR):/spec redocly/openapi-cli lint api/v1/controller/openapi.yaml


.PHONY: integration-tests
integration-tests:
	@echo "run integration-tests"
	$(QUIET) $(MAKE) -C test

.PHONY: unitest-tests
unitest-tests:
	@echo "run unitest-tests"
	$(QUIET) ./ginkgo.sh   \
		--cover --coverprofile=./coverage.out --covermode set  \
		--json-report ./testreport.json \
		-vv  ./pkg/... ./cmd/...
	$(QUIET) go tool cover -html=./coverage.out -o coverage-all.html



.PHONY: manifests
CRD_OPTIONS ?= "crd:crdVersions=v1"
manifests: ## Generate K8s manifests e.g. CRD, RBAC etc.
	@echo "Generate K8s manifests e.g. CRD, RBAC etc."



.PHONY: generate-k8s-api
generate-k8s-api: ## Generate Cilium k8s API client, deepcopy and deepequal Go sources.
	@$(ECHO_CHECK) tools/k8s-code-gen/update-codegen.sh "pkg/k8s/api"
	$(QUIET) tools/k8s-code-gen/update-codegen.sh "pkg/k8s/api"


.PHONY: precheck
precheck: ## Peform build precheck for the source code.
ifeq ($(SKIP_K8S_CODE_GEN_CHECK),"false")
	@$(ECHO_CHECK) tools/k8s-code-gen/verify-codegen.sh
	$(QUIET) tools/k8s-code-gen/verify-codegen.sh
endif

.PHONY: gofmt
gofmt: ## Run gofmt on Go source files in the repository.
	$(QUIET)for pkg in $(GOFILES); do $(GO) fmt $$pkg; done

.PHONY: dev-doctor
dev-doctor:
	$(QUIET)$(GO) version 2>/dev/null || ( echo "go not found, see https://golang.org/doc/install" ; false )
	@$(ECHO_CHECK) contrib/scripts/check-cli.sh
	$(QUIET) contrib/scripts/check-cli.sh



#============ tools ====================

.PHONY: update-authors
update-authors: ## Update AUTHORS file for Cilium repository.
	@echo "Updating AUTHORS file..."
	@echo "The following people, in alphabetical order, have either authored or signed" > AUTHORS
	@echo "off on commits in the Cilium repository:" >> AUTHORS
	@echo "" >> AUTHORS
	@contrib/authorgen/authorgen.sh >> AUTHORS


.PHONY: licenses-all
licenses-all: ## Generate file with all the License from dependencies.
	@$(GO) run ./contrib/licensegen > LICENSE.all || ( rm -f LICENSE.all ; false )

.PHONY: licenses-check
licenses-check:
	@$(ECHO_CHECK) tools/scripts/check-miss-license.sh
	$(QUIET) tools/scripts/check-miss-license.sh

.PHONY: update-go-version
update-go-version: ## Update Go version for all the components
	@echo "GO_MAJOR_AND_MINOR_VERSION=${GO_MAJOR_AND_MINOR_VERSION}"
	@echo "GO_IMAGE_VERSION=${GO_IMAGE_VERSION}"
	# ===== Update Go version for GitHub workflow
	$(QUIET) for fl in $(shell find .github/workflows -name "*.yaml" -print) ; do \
  			sed -i 's/go-version: .*/go-version: ${GO_IMAGE_VERSION}/g' $$fl ; \
  			done
	@echo "Updated go version in GitHub Actions to $(GO_IMAGE_VERSION)"
	# ======= Update Go version in main.go.
	$(QUIET) for fl in $(shell find .  -name main.go -not -path "./vendor/*" -print); do \
		sed -i \
			-e 's|^//go:build go.*|//go:build go${GO_MAJOR_AND_MINOR_VERSION}|g' \
			-e 's|^// +build go.*|// +build go${GO_MAJOR_AND_MINOR_VERSION}|g' \
			$$fl ; \
	done
ifeq (${shell [ -f .travis.yml ] && echo done},done)
	# ====== Update Go version in Travis CI config.
	$(QUIET) sed -i 's/go: ".*/go: "$(GO_VERSION)"/g' .travis.yml
	@echo "Updated go version in .travis.yml to $(GO_VERSION)"
endif
ifeq (${shell [ -d ./test ] && echo done},done)
	# ======= Update Go version in test scripts.
	@echo "Updated go version in test scripts to $(GO_VERSION)"
endif
	# ===== Update Go version in Dockerfiles.
	$(QUIET) $(MAKE) -C images update-golang-image
	@echo "Updated go version in image Dockerfiles to $(GO_IMAGE_VERSION)"



