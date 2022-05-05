include Makefile.defs


.PHONY: lint-golang
lint-golang:
	$(QUIET) scripts/check-go-fmt.sh
	$(QUIET) $(GO_VET)  ./...
	$(QUIET) golangci-lint run



.PHONY: lint-markdown-format
lint-markdown-format:
	@$(CONTAINER_ENGINE) container run --rm \
		--entrypoint sh -v $(ROOT_DIR):/workdir ghcr.io/igorshubovych/markdownlint-cli:latest \
		-c '/usr/local/bin/markdownlint -c /workdir/.github/markdownlint.yaml -p /workdir/.github/markdownlintignore  /workdir/' ; \
		if (($$?==0)) ; then echo "congratulations ,all pass" ; else echo "error, pealse refer <https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md> " ; fi


.PHONY: fix-markdown-format
fix-markdown-format:
	@$(CONTAINER_ENGINE) container run --rm  \
		--entrypoint sh -v $(ROOT_DIR):/workdir ghcr.io/igorshubovych/markdownlint-cli:latest \
		-c '/usr/local/bin/markdownlint -f -c /workdir/.github/markdownlint.yaml -p /workdir/.github/markdownlintignore  /workdir/'

.PHONY: lint-yaml
lint-yaml:
	@$(CONTAINER_ENGINE) container run --rm \
		--entrypoint sh -v $(ROOT_DIR):/data cytopia/yamllint \
		-c '/usr/bin/yamllint -c /data/.github/yamllint-conf.yml /data' ; \
		if (($$?==0)) ; then echo "congratulations ,all pass" ; else echo "error, pealse refer <https://yamllint.readthedocs.io/en/stable/rules.html> " ; fi


.PHONY: lint-markdown-spell
lint-markdown-spell:
	if which mdspell &>/dev/null ; then \
  			mdspell  -r --en-us --ignore-numbers --target-relative .github/.spelling --ignore-acronyms  '**/*.md' '!vendor/**/*.md' ; \
  		else \
			$(CONTAINER_ENGINE) container run --rm \
				--entrypoint bash -v $(ROOT_DIR):/workdir  weizhoulan/spellcheck:latest  \
				-c "cd /workdir ; mdspell  -r --en-us --ignore-numbers --target-relative .github/.spelling --ignore-acronyms  '**/*.md' '!vendor/**/*.md' " ; \
  		fi

.PHONY: lint-markdown-spell-colour
lint-markdown-spell-colour:
	if which mdspell &>/dev/null ; then \
  			mdspell  -r --en-us --ignore-numbers --target-relative .github/.spelling --ignore-acronyms  '**/*.md' '!vendor/**/*.md' ; \
  		else \
			$(CONTAINER_ENGINE) container run --rm -it \
				--entrypoint bash -v $(ROOT_DIR):/workdir  weizhoulan/spellcheck:latest  \
				-c "cd /workdir ; mdspell  -r --en-us --ignore-numbers --target-relative .github/.spelling --ignore-acronyms  '**/*.md' '!vendor/**/*.md' " ; \
  		fi




.PHONY: lint-code-spell
lint-code-spell:
	$(QUIET) if ! which codespell &> /dev/null ; then \
  				echo "try to install codespell" ; \
  				if ! pip3 install codespell ; then \
  					echo "error, miss tool codespell, install it: pip3 install codespell" ; \
  					exit 1 ; \
  				fi \
  			fi ;\
  			codespell --config .github/codespell-config


.PHONY: fix-code-spell
fix-code-spell:
	$(QUIET) if ! which codespell &> /dev/null ; then \
  				echo "try to install codespell" ; \
  				if ! pip3 install codespell ; then \
  					echo "error, miss tool codespell, install it: pip3 install codespell" ; \
  					exit 1 ;\
  				fi \
  			fi; \
  			codespell --config .github/codespell-config  --write-changes


.PHONY: unitest-tests
unitest-tests:
	@echo "run unitest-tests"
	$(QUIET) $(ROOT_DIR)/scripts/ginkgo.sh   \
		--cover --coverprofile=./coverage.out --covermode set  \
		--json-report unitestreport.json \
		-randomize-suites -randomize-all --keep-going  --timeout=1h  -p   --slow-spec-threshold=120s \
		-vv  -r  ./
	$(QUIET) go tool cover -html=./coverage.out -o coverage-all.html

