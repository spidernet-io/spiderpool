#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

if [[ $# -ne 2 ]];then
  echo "Error: This shell-script needs 2 params. Your params is $*"
  exit 1
fi

# ACTION decides which action of this shell you wanna use.
ACTION=$1
# OUTPUT_BASE_DIR defines the output path for swagger generates source codes.
OUTPUT_BASE_DIR=$2

if [[ ! -d ${OUTPUT_BASE_DIR} ]];then
  echo "Error: ${OUTPUT_BASE_DIR} file path doesn't exist!"
  exit 1
fi

API_PATH="${OUTPUT_BASE_DIR}"
YAML_PATH="${API_PATH}/openapi.yaml"
API_CLIENT_PATH="${API_PATH}/client"
API_SERVER_PATH="${API_PATH}/server"
API_MODELS_PATH="${API_PATH}/models"

if [[ ! -f ${YAML_PATH} ]];then
  echo "Error: ${YAML_PATH} spec doesn't exist!"
  exit 1
else
  echo "The chosen API_PATH is '${API_PATH}'"
  echo "The chosen swagger spec is '${YAML_PATH}'"
fi

PROJECT_ROOT_PATH="$(dirname "${BASH_SOURCE[0]}")/../.."
SWAGGER_PKG_PATH=${SWAGGER_PKG_PATH:-$(cd "${PROJECT_ROOT_PATH}"; ls -d -1 ./vendor/github.com/go-swagger/go-swagger 2>/dev/null || echo ../go-swagger)}

# clean up the generated codes
clean() {
  if [[ -d $API_CLIENT_PATH ]];then
    rm -rf $API_CLIENT_PATH
  fi

  if [[ -d $API_SERVER_PATH ]];then
    rm -rf $API_SERVER_PATH
  fi

  if [[ -d $API_MODELS_PATH ]];then
    rm -rf $API_MODELS_PATH
  fi
}

# run swagger with the vendor go-swagger source code main function
RunSwagger() {
  go run ${SWAGGER_PKG_PATH}/cmd/swagger/swagger.go "$@"
}

# validate the given spec
validateSpec() {
  if ! RunSwagger validate ${YAML_PATH}
  then
    echo "Error: failed validate spec: ${YAML_PATH}"
    exit 1
  fi
}

# generate C/S source codes with the spec
generateCode() {
  clean

  if ! RunSwagger generate server "$@" \
        -s server -a restapi \
        -f ${YAML_PATH} \
        --target ${API_PATH} \
        --exclude-main \
        --default-scheme=unix \
        -r ${PROJECT_ROOT_PATH}/tools/spdx-copyright-header.txt
  then
    echo "Error: Failed run swagger to generate server for yaml '${YAML_PATH}' "
    exit 1
  fi

  if ! RunSwagger generate client "$@" \
        -a restapi \
        -f ${YAML_PATH} \
        --target ${API_PATH} \
        -r ${PROJECT_ROOT_PATH}/tools/spdx-copyright-header.txt
  then
    echo "Error: Failed run swagger to generate server for yaml '${YAML_PATH}' "
    exit 1
  fi
}

# verify the current source codes with the spec to make sure whether the source codes is out of date
verify() {
  DIFFROOT=${API_PATH}
  TMP_DIFFROOT="${PROJECT_ROOT_PATH}/_openapi_tmp/"

  if [ -d ${TMP_DIFFROOT} ];then
    rm -rf ${TMP_DIFFROOT}
  fi

  mkdir -p "${TMP_DIFFROOT}"
  cp -a "${DIFFROOT}"/* "${TMP_DIFFROOT}"

  generateCode -q
  echo "diffing ${DIFFROOT} against freshly generated codegen"
  ret=0
  diff -Naupr "${DIFFROOT}" "${TMP_DIFFROOT}" || ret=$?
  cp -a "${TMP_DIFFROOT}"/* "${DIFFROOT}"
  rm -rf "${TMP_DIFFROOT}"
  if [[ $ret -eq 0 ]];then
    echo "${DIFFROOT} up to date."
  else
    echo "Error: ${DIFFROOT} is out of date! Please run 'make openapi-code-gen'"
    exit 1
  fi
}

case ${ACTION} in
  validate)
    echo -e "Your action is 'validate'. Going to validate the '${YAML_PATH}' spec...\n"
    validateSpec
    ;;
  generate)
    echo -e "Your action is 'generate'. Going to generate source codes with '${YAML_PATH}' spec...\n"
    generateCode
    ;;
  clean)
    echo -e "Your action is 'clean'. Going to clean '${API_PATH}' legacy...\n"
    clean
    ;;
  verify)
    echo -e "Your action is 'verify'. Going to verify the '${API_PATH}' source codes with '${YAML_PATH}' spec...\n"
    verify
    ;;
  *)
    echo "Error: Unidentifiable param: '${ACTION}', please pass the correct 1st param in 'validate|generate|clean|verify'"
    exit 1
esac

