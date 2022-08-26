#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT=$(git rev-parse --show-toplevel)
APIS_PKG="pkg/k8s/apis"
OUTPUT_PKG="pkg/k8s/client"

API_DIFFROOT="${PROJECT_ROOT}/${APIS_PKG}"
CLIENT_DIFFROOT="${PROJECT_ROOT}/${OUTPUT_PKG}"

_tmp="${PROJECT_ROOT}/_tmp"
TMP_API_DIFFROOT="${_tmp}/api"
TMP_CLIENT_DIFFROOT="${_tmp}/client"

cleanup() {
  rm -rf "${_tmp}"
}
trap "cleanup" EXIT SIGINT

cleanup

mkdir -p "${_tmp}"
cp -a "${API_DIFFROOT}" "${TMP_API_DIFFROOT}"
cp -a "${CLIENT_DIFFROOT}" "${TMP_CLIENT_DIFFROOT}"

"${PROJECT_ROOT}/tools/scripts/update-codegen.sh"

diff_code() {
    local diff=$1
    local tmp=$2

    echo "diffing ${diff} against freshly generated codegen"
    ret=0
    diff -Naupr "${diff}" "${tmp}" || ret=$?
    cp -a "${tmp}"/* "${diff}"
    if [[ $ret -eq 0 ]]; then
      echo "${diff} up to date."
    else
      echo "${diff} is out of date! Please run 'make codegen'"
      exit 1
    fi
}

diff_code ${API_DIFFROOT} ${TMP_API_DIFFROOT}
diff_code ${CLIENT_DIFFROOT} ${TMP_CLIENT_DIFFROOT}
