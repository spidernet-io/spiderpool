#!/usr/bin/env bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

CURRENT_FILENAME=`basename $0`
CURRENT_DIR_PATH=$(cd `dirname $0`; pwd)

export PROJECT_ROOT_PATH="${CURRENT_DIR_PATH}/.."
export GINKGO_PKG_PATH=${GINKGO_PKG_PATH:-${PROJECT_ROOT_PATH}/vendor/github.com/onsi/ginkgo/v2/ginkgo/main.go}

if [ -f "$GINKGO_PKG_PATH" ] ; then
  go run $GINKGO_PKG_PATH -vv -r ${CURRENT_DIR_PATH} $@
else
  echo "failed to find ginkgo vendor $GINKGO_PKG_PATH "
  exit 1
fi

