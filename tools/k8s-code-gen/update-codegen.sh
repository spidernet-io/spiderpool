#!/usr/bin/env bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

CURRENT_FILENAME=`basename $0`
CURRENT_DIR_PATH=$(cd `dirname $0`; pwd)

# https://github.com/kubernetes/code-generator/issues/106
# 问题1：client-set 工具在寻找源码时，会把 项目下的 vendor 作为根目录来寻找，所以，我们做了copy操作，请保障 vednor 目录下不会有重名的目录 ${PROJECT_ROOT_PATH}/vendor/${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}
# 问题2：除了deepcopy，其它的工具 会把 项目下的 ${MODUEL_NAME} 作为根目录 来 生成代码， 所以，我们最后其实做了 move 操作 ，请保障 项目下不会有重名目录 ${MODUEL_NAME}

# source: pkg/k8s/api/*/v1
# ./CURRENT_FILENAME  "pkg/k8s/api"


# can not use absolute path
export PROJECT_ROOT_PATH="$(dirname "${BASH_SOURCE[0]}")/../.."
export CODEGEN_PKG_PATH=${CODEGEN_PKG_PATH:-$(cd "${PROJECT_ROOT_PATH}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}
export MODUEL_NAME=$(cat ${PROJECT_ROOT_PATH}/go.mod | egrep "module[[:space:]][^[:space:]]*" | awk '{print $2}')

INPUT_PATH_BASE=${1}
[ -z "$INPUT_PATH_BASE" ] && echo "error, miss package path" && exit 1
[ ! -d "${PROJECT_ROOT_PATH}/${INPUT_PATH_BASE}" ] && echo "error, miss package path ${PROJECT_ROOT_PATH}/${INPUT_PATH_BASE} " && exit 2

# 填写如下信息
export INPUT_PATH_BASE=$INPUT_PATH_BASE
#export API_OPERATOR_NAME="welancontroller"
#export API_VERSION="v1"
#export OUTPUT_PATH_BASE="${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}/generated"



#=================
echo "package name: $MODUEL_NAME"
if [ -z "$MODUEL_NAME" ] ; then
  echo "error, failed to find MODUEL_NAME "
  exit 1
fi

DEBUG=false

#=================

CONTROLLERS=$( ls $INPUT_PATH_BASE )
for IT in $CONTROLLERS ; do
  # welancontroller
  export API_OPERATOR_NAME="$IT"
  # v1
  VER=$( ls ${INPUT_PATH_BASE}/${IT} )
  export API_VERSION="$VER"
  export OUTPUT_PATH_BASE="${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}/generated"

  # when generate "informer", "lister" and "client" package must exist
  bash ${CURRENT_DIR_PATH}/generate-groups.sh \
    "all" \
    ${DEBUG} \
    --output-base "${PROJECT_ROOT_PATH}" \
    --go-header-file ${CURRENT_DIR_PATH}/boilerplate.go.txt
done




