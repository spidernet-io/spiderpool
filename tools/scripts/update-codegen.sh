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
go_path="${PROJECT_ROOT}/_go"

cleanup() {
  rm -rf ${go_path}
  rm -f ${LICENSE_FILE}
}
trap "cleanup" EXIT SIGINT
cleanup

touch ${LICENSE_FILE}

while read -r line || [[ -n ${line} ]]
do
    echo "// ${line}" >>${LICENSE_FILE}
done < ${SPDX_COPYRIGHT_HEADER}

APIS_PKG="pkg/k8s/apis"
OUTPUT_PKG="pkg/k8s/client"
GROUPS_WITH_VERSIONS="spiderpool.spidernet.io:v2beta1"

echo "change directory: ${PROJECT_ROOT}"
cd "${PROJECT_ROOT}"

go_pkg="${go_path}/src/github.com/spidernet-io/spiderpool"
go_pkg_dir=$(dirname "${go_pkg}")
mkdir -p "${go_pkg_dir}"

if [[ ! -e "${go_pkg_dir}" || "$(readlink "${go_pkg_dir}")" != "${PROJECT_ROOT}" ]]; then
  ln -snf "${PROJECT_ROOT}" "${go_pkg_dir}"
fi
export GOPATH="${go_path}"

bash ${PROJECT_ROOT}/${CODEGEN_PKG}/generate-groups.sh "client,informer,lister" \
  ${MODULE_NAME}/${OUTPUT_PKG} \
  ${MODULE_NAME}/${APIS_PKG} \
  ${GROUPS_WITH_VERSIONS} \
  --go-header-file ${LICENSE_FILE}

rm -f ${LICENSE_FILE}
