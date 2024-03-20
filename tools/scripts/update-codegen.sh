#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

# Copyright 2023 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This presents several functions for packages which want to use kubernetes
# code-generation tools.

set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT=$(git rev-parse --show-toplevel)
CODEGEN_PKG=${CODEGEN_PKG_PATH:-$(cd ${PROJECT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}
MODULE_NAME=$(cat ${PROJECT_ROOT}/go.mod | grep -e "module[[:space:]][^[:space:]]*" | awk '{print $2}')

SPDX_COPYRIGHT_HEADER="${PROJECT_ROOT}/tools/spdx-copyright-header.txt"
LICENSE_FILE="${PROJECT_ROOT}/tools/boilerplate.go.txt"

cleanup() {
  rm -f ${LICENSE_FILE}
}
trap "cleanup" EXIT SIGINT
cleanup

# generate license file
touch ${LICENSE_FILE}
while read -r line || [[ -n ${line} ]]
do
    echo "// ${line}" >>${LICENSE_FILE}
done < ${SPDX_COPYRIGHT_HEADER}

APIS_PKG="pkg/k8s/apis"
OUTPUT_PKG="pkg/k8s/client"

source "${PROJECT_ROOT}/${CODEGEN_PKG}/kube_codegen.sh"

kube::codegen::gen_client\
    --with-watch \
    --output-dir "${PROJECT_ROOT}/${OUTPUT_PKG}" \
    --output-pkg "${MODULE_NAME}/${OUTPUT_PKG}" \
    --boilerplate ${LICENSE_FILE} \
    "${PROJECT_ROOT}/${APIS_PKG}"

rm -f ${LICENSE_FILE}
