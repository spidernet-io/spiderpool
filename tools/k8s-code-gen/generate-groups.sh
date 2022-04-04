#!/usr/bin/env bash

# Copyright 2017 Authors of Kubernetes
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail
#set -x

# generate-groups generates everything for a project with external types only, e.g. a project based
# on CustomResourceDefinitions.

if [ "$#" -lt 4 ] || [ "${1}" == "--help" ]; then
  cat <<EOF
Usage: $(basename "$0") <generators> <debug> ...
  <generators>        the generators comma separated to run (deepcopy,defaulter,client,lister,informer) or "all".
  <debug>    true or false
EOF
  exit 0
fi

GENS="$1"
DEBUG="$2"

if [ "$DEBUG"x == "true"x ]; then
  DEBUG="-v 10"
else
  DEBUG=""
fi

shift 2

CLIENTSET_PKG_NAME=${CLIENTSET_PKG_NAME:-"clientset"}
CLIENTSET_NAME_VERSIONED=${CLIENTSET_NAME_VERSIONED:-"versioned"}
LISTERS_PKG_NAME=${LISTERS_PKG_NAME:-"listers"}
INFORMER_PKG_NAME=${INFORMER_PKG_NAME:-"informers"}


# https://github.com/kubernetes/code-generator/issues/106
# the bug will find api in "vendor" directory
\rm -rf ${PROJECT_ROOT_PATH}/vendor/${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}
mkdir -p ${PROJECT_ROOT_PATH}/vendor/${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}
cp -rf ${PROJECT_ROOT_PATH}/${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}  \
    ${PROJECT_ROOT_PATH}/vendor/${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}/..


if [ "${GENS}" = "all" ] || grep -qw "deepcopy" <<<"${GENS}"; then
  echo "Generating deepcopy funcs to ${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION} "
  go run ${CODEGEN_PKG_PATH}/cmd/deepcopy-gen/main.go \
      --input-dirs "${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}" -O zz_generated.deepcopy "$@" $DEBUG
fi

if [ "${GENS}" = "all" ] || grep -qw "client" <<<"${GENS}"; then

  echo "Generating clientset to ${OUTPUT_PATH_BASE}/${CLIENTSET_PKG_NAME} "
  go run ${CODEGEN_PKG_PATH}/cmd/client-gen/main.go \
      --clientset-name "${CLIENTSET_NAME_VERSIONED:-versioned}" \
      --input-base "" \
      --input "${MODUEL_NAME}/${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}"  \
      --output-package "${MODUEL_NAME}/${OUTPUT_PATH_BASE}/${CLIENTSET_PKG_NAME}" \
      --output-base "${PROJECT_ROOT_PATH}" \
      "$@" $DEBUG

  # https://github.com/kubernetes/code-generator/issues/106
  # move the directory
  mkdir -p ${PROJECT_ROOT_PATH}/${OUTPUT_PATH_BASE}/
  cp -rf ${PROJECT_ROOT_PATH}/${MODUEL_NAME}/${OUTPUT_PATH_BASE}/*  \
     ${PROJECT_ROOT_PATH}/${OUTPUT_PATH_BASE}/
  \rm -rf ${PROJECT_ROOT_PATH}/${MODUEL_NAME}

fi

if [ "${GENS}" = "all" ] || grep -qw "lister" <<<"${GENS}"; then
  echo "Generating listers to ${OUTPUT_PATH_BASE}/${LISTERS_PKG_NAME} "
  go run ${CODEGEN_PKG_PATH}/cmd/lister-gen/main.go \
       --input-dirs "${MODUEL_NAME}/${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}" \
       --output-package "${MODUEL_NAME}/${OUTPUT_PATH_BASE}/${LISTERS_PKG_NAME}" "$@" $DEBUG

  # https://github.com/kubernetes/code-generator/issues/106
  # move the directory
  mkdir -p ${PROJECT_ROOT_PATH}/${OUTPUT_PATH_BASE}/
  cp -rf ${PROJECT_ROOT_PATH}/${MODUEL_NAME}/${OUTPUT_PATH_BASE}/*  \
     ${PROJECT_ROOT_PATH}/${OUTPUT_PATH_BASE}/
  \rm -rf ${PROJECT_ROOT_PATH}/${MODUEL_NAME}
fi

if [ "${GENS}" = "all" ] || grep -qw "informer" <<<"${GENS}"; then
  echo "Generating informers to ${OUTPUT_PATH_BASE}/${INFORMER_PKG_NAME} "
  go run ${CODEGEN_PKG_PATH}/cmd/informer-gen/main.go \
           --input-dirs "${MODUEL_NAME}/${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}" \
           --versioned-clientset-package "${MODUEL_NAME}/${OUTPUT_PATH_BASE}/${CLIENTSET_PKG_NAME:-clientset}/${CLIENTSET_NAME_VERSIONED:-versioned}" \
           --listers-package "${MODUEL_NAME}/${OUTPUT_PATH_BASE}/${LISTERS_PKG_NAME}" \
           --output-package "${MODUEL_NAME}/${OUTPUT_PATH_BASE}/${INFORMER_PKG_NAME}" \
           "$@" $DEBUG

  # https://github.com/kubernetes/code-generator/issues/106
  # move the directory
  mkdir -p ${PROJECT_ROOT_PATH}/${OUTPUT_PATH_BASE}/
  cp -rf ${PROJECT_ROOT_PATH}/${MODUEL_NAME}/${OUTPUT_PATH_BASE}/*  \
     ${PROJECT_ROOT_PATH}/${OUTPUT_PATH_BASE}/
  \rm -rf ${PROJECT_ROOT_PATH}/${MODUEL_NAME}
fi

# https://github.com/kubernetes/code-generator/issues/106
\rm -rf ${PROJECT_ROOT_PATH}/vendor/${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}/
