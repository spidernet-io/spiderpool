#!/usr/bin/env bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT=$(dirname ${BASH_SOURCE[0]})/../..
CODEGEN_PKG=${CODEGEN_PKG_PATH:-$(cd ${PROJECT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}
MODULE_NAME=$(cat ${PROJECT_ROOT}/go.mod | egrep "module[[:space:]][^[:space:]]*" | awk '{print $2}')

APIS_PKG="pkg/k8s/apis"
OUTPUT_PKG="pkg/k8s/client"
GROUPS_WITH_VERSIONS="spiderpool:v1"

bash ${PROJECT_ROOT}/${CODEGEN_PKG}/generate-groups.sh all \
  ${MODULE_NAME}/${OUTPUT_PKG} \
  ${MODULE_NAME}/${APIS_PKG} \
  ${GROUPS_WITH_VERSIONS} \
  --output-base . \
  --go-header-file ${PROJECT_ROOT}/tools/boilerplate.go.txt
