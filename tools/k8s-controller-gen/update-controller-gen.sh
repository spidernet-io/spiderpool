#!/usr/bin/env bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

# CONST
PROJECT_ROOT=$(dirname ${BASH_SOURCE[0]})/../..
CODEGEN_PKG=${CODEGEN_PKG_PATH:-$(cd ${PROJECT_ROOT}; ls -d -1 ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen 2>/dev/null || echo ../controller-gen)}

# ENV
# Defines the output path for the artifacts controller-gen generates
OUTPUT_BASE_DIR=${OUTPUT_BASE_DIR:-${PROJECT_ROOT}/charts/spiderpool}
# Defines tmp path of the current artifacts for diffing
OUTPUT_TMP_DIR=${OUTPUT_TMP_DIR:-${PROJECT_ROOT}/_controller_gen_tmp}
# Defines the output path of the latest artifacts for diffing
OUTPUT_DIFF_DIR=${OUTPUT_DIFF_DIR:-${PROJECT_ROOT}/_controller_gen_diff}



controller-gen() {
  go run ${PROJECT_ROOT}/${CODEGEN_PKG}/main.go "$@"
}

manifests_gen() {
  output_dir=$1

  controller-gen \
  crd webhook rbac:roleName="spiderpool" \
  paths="${PWD}/${PROJECT_ROOT}/pkg/k8s/apis/v1" \
  output:crd:artifacts:config="${output_dir}/crds" \
  output:webhook:artifacts:config="${output_dir}/webhook" \
  output:rbac:artifacts:config="${output_dir}/rbac"
}

deepcopy_gen() {
  tmp_header_file=${PROJECT_ROOT}/tools/boilerplate.go.txt
  cat ${PROJECT_ROOT}/tools/spdx-copyright-header.txt | sed -E 's?(.*)?// \1?' > ${tmp_header_file}

  controller-gen \
  object:headerFile="${tmp_header_file}" \
  paths="${PWD}/${PROJECT_ROOT}/pkg/k8s/apis/v1"

  rm -f ${tmp_header_file}
}

manifests_verify() {
  diff_root=${OUTPUT_TMP_DIR}
  tmp_diff_root=${OUTPUT_DIFF_DIR}

  # Clean up tmp dir
  if [ -d ${diff_root} ];then
    rm -rf ${diff_root}
  fi

  if [ -d ${tmp_diff_root} ];then
    rm -rf ${tmp_diff_root}
  fi

  # Aggregate the artifacts currently in use
  mkdir -p ${diff_root}
  if [ "$(ls -A ${OUTPUT_BASE_DIR}/crds)" ]; then
    cp -ra ${OUTPUT_BASE_DIR}/crds ${diff_root}
  fi

  if [ "$(ls -A ${OUTPUT_BASE_DIR}/webhook)" ]; then
    cp -ra ${OUTPUT_BASE_DIR}/webhook ${diff_root}
  fi

  if [ "$(ls -A ${OUTPUT_BASE_DIR}/rbac)" ]; then
    cp -ra ${OUTPUT_BASE_DIR}/rbac ${diff_root}
  fi

  # Generator the latest artifacts
  manifests_gen ${tmp_diff_root}

  # Diff
  ret=0
  diff -Naupr ${diff_root} ${tmp_diff_root} || ret=$?
  rm -rf ${diff_root} ${tmp_diff_root}

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
}

main "$*"