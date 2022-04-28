#!/usr/bin/env bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

# CONST
PROJECT_ROOT=$(dirname ${BASH_SOURCE[0]})/../..
CONTROLLER_GEN_TMP_DIR=${CONTROLLER_GEN_TMP_DIR:-${PROJECT_ROOT}/.controller_gen_tmp}
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${PROJECT_ROOT}; ls -d -1 ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen 2>/dev/null || echo ../controller-gen)}

# ENV
# Defines the output path for the artifacts controller-gen generates
OUTPUT_BASE_DIR=${OUTPUT_BASE_DIR:-${PROJECT_ROOT}/charts/spiderpool}
# Defines tmp path of the current artifacts for diffing
OUTPUT_TMP_DIR=${OUTPUT_TMP_DIR:-${CONTROLLER_GEN_TMP_DIR}/old}
# Defines the output path of the latest artifacts for diffing
OUTPUT_DIFF_DIR=${OUTPUT_DIFF_DIR:-${CONTROLLER_GEN_TMP_DIR}/new}



controller-gen() {
  go run ${PROJECT_ROOT}/${CODEGEN_PKG}/main.go "$@"
}

manifests_gen() {
  output_dir=$1

  controller-gen \
  crd webhook rbac:roleName="spiderpool" \
  paths="${PWD}/${PROJECT_ROOT}/pkg/k8s/apis/v1;${PWD}/${PROJECT_ROOT}/pkg/webhook" \
  output:crd:artifacts:config="${output_dir}/crds" \
  output:webhook:artifacts:config="${output_dir}/webhook" \
  output:rbac:artifacts:config="${output_dir}/rbac"
}

deepcopy_gen() {
  tmp_header_file=${CONTROLLER_GEN_TMP_DIR}/boilerplate.go.txt
  cat ${PROJECT_ROOT}/tools/spdx-copyright-header.txt | sed -E 's?(.*)?// \1?' > ${tmp_header_file}

  controller-gen \
  object:headerFile="${tmp_header_file}" \
  paths="${PWD}/${PROJECT_ROOT}/pkg/k8s/apis/v1"
}

manifests_verify() {
  # Aggregate the artifacts currently in use
  mkdir -p ${OUTPUT_TMP_DIR}
  if [ "$(ls -A ${OUTPUT_BASE_DIR}/crds)" ]; then
    cp -ra ${OUTPUT_BASE_DIR}/crds ${OUTPUT_TMP_DIR}
  fi

  if [ "$(ls -A ${OUTPUT_BASE_DIR}/webhook)" ]; then
    cp -ra ${OUTPUT_BASE_DIR}/webhook ${OUTPUT_TMP_DIR}
  fi

  if [ "$(ls -A ${OUTPUT_BASE_DIR}/rbac)" ]; then
    cp -ra ${OUTPUT_BASE_DIR}/rbac ${OUTPUT_TMP_DIR}
  fi

  # Generator the latest artifacts
  manifests_gen ${OUTPUT_DIFF_DIR}

  # Diff
  ret=0
  diff -Naupr ${OUTPUT_TMP_DIR} ${OUTPUT_DIFF_DIR} || ret=$?

  if [[ $ret -eq 0 ]];then
    echo "The Artifacts is up to date."
  else
    echo "Error: The Artifacts is out of date! Please run 'make manifests'."
    exit 1
  fi
}

help() {
    echo "help"
}

main() {
  if [ -d ${CONTROLLER_GEN_TMP_DIR} ];then
    rm -rf ${CONTROLLER_GEN_TMP_DIR}
  fi
  mkdir -p ${CONTROLLER_GEN_TMP_DIR}

  case ${1:-none} in
    manifests)
      manifests_gen ${OUTPUT_BASE_DIR}
      ;;
    deepcopy)
      deepcopy_gen
      ;;
    verify)
      manifests_verify
      ;;
    *|help|-h|--help)
      help
      ;;
  esac

  # Clean up controller-gen tmp dir
  rm -rf ${CONTROLLER_GEN_TMP_DIR}
}

main "$*"