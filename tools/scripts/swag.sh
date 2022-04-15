#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

fn=$1
dir=$2

API_PATH="${dir}"
API_YAML_PATH="${API_PATH}/swagger.yml"
API_CLIENT_PATH="${API_PATH}/client"
API_SERVER_PATH="${API_PATH}/server"
API_MODELS_PATH="${API_PATH}/models"

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
  RunSwagger validate ${API_YAML_PATH} 2>&1
  result=`swagger validate "${API_YAML_PATH}" 2>&1`
  if `echo ${result} | grep "invalid" > /dev/null 2>&1`; then
    exit 1
  else
    exit 0
  fi
}

# generate C/S source codes with the spec
generateCode() {
  clean

  if ! RunSwagger generate server "$@" \
        -s server -a restapi \
        -f ${API_YAML_PATH} \
        --target ${API_PATH} \
        --exclude-main \
        --default-scheme=unix \
        -r ${PROJECT_ROOT_PATH}/tools/spdx-copyright-header.txt
  then
    exit 1
  fi

  if ! RunSwagger generate client "$@" \
        -a restapi \
        -f ${API_YAML_PATH} \
        --target ${API_PATH} \
        -r ${PROJECT_ROOT_PATH}/tools/spdx-copyright-header.txt
  then
    exit 1
  fi
}

# verify the current source codes with the spec to make sure whether the source codes is out of date
verify() {
  DIFFROOT=${API_PATH}
  TMP_DIFFROOT="${API_PATH}/../_tmp/"

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
    echo "Error! ${DIFFROOT} is out of date. Please run 'make openapi-code-gen'"
    exit 1
  fi
}

if [[ $fn == "validate" ]];then
  validateSpec
fi

if [[ $fn == "generate" ]];then
  generateCode
fi

if [[ $fn == "clean" ]];then
  clean
fi

if [[ $fn == "verify" ]];then
  verify
fi

