# Submit Pull request

A pull request will be checked by following workflow, which is the required condition for merging

## action: PR should be signed off

## action: check yaml files

If this check fails, it could refer to the [yaml rule](https://yamllint.readthedocs.io/en/stable/rules.html)

Once the issue is fixed , it could be verified on local host by command ` make lint-yaml `

notice: for ignoring yaml rule, it could be add to .github/yamllint-conf.yml

## action: check golang source code

It checks the following against any updated golang file

* mod dependency updated, golangci-lint, gofmt updated, go vet, use internal lock pkg

* comment `// TODO` should follow format: `// TODO (AuthorName) ...`, which easy to trace the owner of the remaining job

* unitest and upload coverage to codecov

* each golang test file should mark ginkgo label

## action: check license

Any golang or shell file should be licensed correctly.

## action: check markdown file

* check markdown format, if fails, it could refer to the [Markdown Rule](https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md)

    It could be tested on local machine with command `make lint-markdown-format`

    It could try to fix it on local machine with command `make fix-markdown-format`

    For ignoring case, it could be added to .github/markdownlint.yaml

* check markdown spell error.
  
    It could be tested on local machine with command `make lint-markdown-spell-colour`.

    For ignoring case, it could be added it to .github/.spelling

## action: lint yaml file

if it fails, the reason could refer to <https://yamllint.readthedocs.io/en/stable/rules.html>

It could be tested on local machine with command `make lint-yaml`

## action: lint chart

## action: lint openapi.yaml

## action: check code spell

Any code spell error of golang files will be checked.

It could be checked on local machine with command `make lint-code-spell`.

It could be auto fixed on local machine with command `make fix-code-spell`.

For ignored error case, please edit .github/codespell-ignorewords and make sure all letters should be lower-case
