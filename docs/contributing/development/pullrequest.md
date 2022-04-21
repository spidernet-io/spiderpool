# PR Management

after submitting a pull request, the reviewer is assigned by the CODEOWNERS.

if a PR lasts stale longer than 60 days, label it with "pr/stale" and then close it after 14 days

a PR may trigger following workflows, it should meet all of them for merging PR

## action: check signed off

if a pr is not signed off, a robot comment will be update to prompted

## action: check yaml file issue

you could find the reported issue description <https://yamllint.readthedocs.io/en/stable/rules.html>

use following to find the issue on you local machine

```
make lint-yaml
```

notice: for some ignoring case, you could add to .github/yamllint-conf.yml

## action: go source code check

any go file updated, will check it with following:

1 mod dependency updated, golangci-lint, gofmt updated, go vet, use internal lock pkg

2 code quality check, like codeql and gokart

3 build binary

4 unitest and upload coverage to codecov

5 action: lint license

6 each golang test file should mark ginkgo label

## action: license

any go or shell file should be licensed

if it belongs to spiderpool, could set it as

```
// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
```

## action: lint markdown file

check markdown file format , spell and best practice.

if it fails, the reason could refer to <https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md>

you could test it on local machine with following command

```
make lint-markdown-format
```

you could try to justify it on local machine with following command. For ignoring case, you could add to .github/markdownlint.yaml

```
make fix-markdown-format
```

you could test it on local machine with following command. For ignoring case, you could add it to .github/.spelling

```
make lint-markdown-spell-colour
```

## action: lint yaml file

check yaml file format and best practice.

if it fails, the reason could refer to <https://yamllint.readthedocs.io/en/stable/rules.html>

you could test it on local machine with following command

```
make lint-yaml
```

## action: lint chart

any update about chart file under '/charts'

## action: lint openapi.yaml

any update about openapi.yaml, will be checked for the yaml validation

## action: other github APP

check from <https://www.codefactor.io>

## action: check code spell error

check on local machine

```
make lint-code-spell
```

fix on local machine. For ignored case, please edit .github/codespell-ignorewords and make sure all letters should be lower-case

```
make fix-code-spell
```

## need review

any PR need 2 review, if meet, will auto label it with "pr/approved" and "pr/need-release-label"
