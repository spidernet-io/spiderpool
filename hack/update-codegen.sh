#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail


# https://github.com/kubernetes/code-generator/issues/106
# 问题1：client-set 工具在寻找源码时，会把 项目下的 vendor 作为根目录来寻找，所以，我们做了copy操作，请保障 vednor 目录下不会有重名的目录 ${PROJECT_ROOT_PATH}/vendor/${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}
# 问题2：除了deepcopy，其它的工具 会把 项目下的 ${MODUEL_NAME} 作为根目录 来 生成代码， 所以，我们最后其实做了 move 操作 ，请保障 项目下不会有重名目录 ${MODUEL_NAME}

# 填写如下信息
# source: pkg/apis/spiderpool/v1
export INPUT_PATH_BASE=${INPUT_PATH_BASE:-"pkg/apis"}
export API_OPERATOR_NAME=${API_OPERATOR_NAME:-"spiderpool"}
export API_VERSION=${API_VERSION:-"v1"}
export OUTPUT_PATH_BASE="${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}/generated"



# can not use absolute path
export PROJECT_ROOT_PATH="$(dirname "${BASH_SOURCE[0]}")/.."
export CODEGEN_PKG_PATH=${CODEGEN_PKG_PATH:-$(cd "${PROJECT_ROOT_PATH}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}
export MODUEL_NAME=$(cat ${PROJECT_ROOT_PATH}/go.mod | egrep "module[[:space:]][^[:space:]]*" | awk '{print $2}')

#=================
echo "package name: $MODUEL_NAME"
if [ -z "$MODUEL_NAME" ] ; then
  echo "error, failed to find MODUEL_NAME "
  exit 1
fi

DEBUG=false
# when generate "informer", "lister" and "client" package must exist
bash ${PROJECT_ROOT_PATH}/hack/generate-groups.sh \
  "all" \
  ${DEBUG} \
  --output-base "${PROJECT_ROOT_PATH}" \
  --go-header-file "${PROJECT_ROOT_PATH}"/hack/boilerplate.go.txt


