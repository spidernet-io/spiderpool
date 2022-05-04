#!/bin/sh

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

# could set proxy for accessing k8s
# export https_proxy=http://127.0.0.1:7890 http_proxy=http://127.0.0.1:7890

#set -euo pipefail
set -x

VERSION=${1#"v"}
if [ -z "$VERSION" ]; then
    echo "Must specify version!"
    exit 1
fi

echo "try to get k8s go.mod"
MODS=($(
    curl -sS https://raw.githubusercontent.com/kubernetes/kubernetes/v${VERSION}/go.mod |
    sed -n 's|.*k8s.io/\(.*\) => ./staging/src/k8s.io/.*|k8s.io/\1|p'
))

echo "begin to go edit"
for MOD in "${MODS[@]}"; do
    V=$( go mod download -json "${MOD}@kubernetes-${VERSION}" | sed -n 's?.*"Version": "\(.*\)".*?\1?p' )
    echo "mod=${MOD}, version=${V}"
    go mod edit "-replace=${MOD}=${MOD}@${V}"
done

echo "begin to go get "
go get "k8s.io/kubernetes@v${VERSION}"
