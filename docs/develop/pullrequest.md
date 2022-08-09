# Submit Pull Request

A pull request will be checked by following workflow, which is required for merging.

## Action: your PR should be signed off

When you commit your modification, add `-s` in your commit command:

```
git commit -s
```

## Action: check yaml files

If this check fails, see the [yaml rule](https://yamllint.readthedocs.io/en/stable/rules.html).

Once the issue is fixed, it could be verified on your local host by command `make lint-yaml`.

Note: To ignore a yaml rule, you can add it into `.github/yamllint-conf.yml`.

## Action: check golang source code

It checks the following items against any updated golang file.

* Mod dependency updated, golangci-lint, gofmt updated, go vet, use internal lock pkg

* Comment `// TODO` should follow the format: `// TODO (AuthorName) ...`, which easy to trace the owner of the remaining job

* Unitest and upload coverage to codecov

* Each golang test file should mark ginkgo label

## Action: check licenses

Any golang or shell file should be licensed correctly.

## Action: check markdown file

* Check markdown format, if fails, See the [Markdown Rule](https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md)

  You can test it on your local machine with the command `make lint-markdown-format`.

  You can fix it on your local machine with the command `make fix-markdown-format`.

  If you believe it can be ignored, you can add it to `.github/markdownlint.yaml`.

* Check markdown spell error.
  
  You can test it with the command `make lint-markdown-spell-colour`.

  If you believe it can be ignored, you can add it to `.github/.spelling`.

## Action: lint yaml file

If it fails, see <https://yamllint.readthedocs.io/en/stable/rules.html> for reasons.

You can test it on your local machine with the command `make lint-yaml`.

## Action: lint chart

## Action: lint openapi.yaml

## Action: check code spell

Any code spell error of golang files will be checked.

You can check it on your local machine with the command `make lint-code-spell`.

It could be automatically fixed on your local machine with the command `make fix-code-spell`.

If you believe it can be ignored, edit `.github/codespell-ignorewords` and make sure all letters are lower-case.
