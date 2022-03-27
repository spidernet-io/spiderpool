# PR Management

after submitting a pull request, the reviewer is assigned by the CODEOWNERS. 

if a PR lasts stale longer than 60 days, label it with "pr/stale" and then close it after 14 days

a PR may trigger following workflows, it should meet all of them for merging PR

### action: check signed off

if a pr is not signed off, a robot comment will be update to prompted

### action: check markdown file issue 

you could find the reported issue description <https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md>

use following to find the issue on you local machine
```
make lint-markdown
```

use following the fix issue on you local machine
```
make fix-markdown
```

### action: check yaml file issue 

you could find the reported issue description <https://yamllint.readthedocs.io/en/stable/rules.html>

use following to find the issue on you local machine
```
make lint-yaml
```

### action: build CI image

With cache acceleration, build two ci image and push to ghcr

(1) ****-ci:${ref} : the normal image

(2) ****-ci:${ref}-rate : image who turns on 'go race' and 'deadlock detect'

the CI will clean ci images at interval

### action: lint CodeQL

any go file updated, will check it with CodeQL

### action: lint golang

any go file updated, will check it with following:

(1) golangci-lint and go vet

(2) gokart

(3) whether update go.mod and vendor

(4) whether gofmt the code

(5) wheterh use the pkg/lock

### action: lint license

any go or shell file should be licensed

if it belongs to spiderpool, could set it as

```
// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
```

### action: lint markdown file

check markdonw file format and best practice.

if it fails, the reason could refer to <https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md>

you could test it on local machine with following command

```
make lint-markdown
```

you could tyr to justify it on local machine with following command

```
make fix-markdown
```

### action: lint yaml file

check yaml file format and best practice.

if it fails, the reason could refer to <https://yamllint.readthedocs.io/en/stable/rules.html>

you could test it on local machine with following command

```
make lint-yaml
```

### action: lint chart 

any update about chart file under '/charts', will trigger this 

### need review 

any PR need 2 review, if meet, will auto label it with "pr/approved" and "pr/need-release-label"

