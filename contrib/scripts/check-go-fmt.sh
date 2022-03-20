#!/usr/bin/env bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

diff="$(find . \
        ! \( -path './vendor' -prune \) \
        ! \( -path './_build' -prune \) \
        ! \( -path './.git' -prune \) \
        ! \( -path '*.validate.go' -prune \) \
        -type f -name '*.go' | \
        xargs gofmt -d -l -s )"

if [ -n "$diff" ]; then
	echo "Unformatted Go source code:"
	echo "$diff"
	exit 1
fi
echo "format of Go source code is right"
exit 0
