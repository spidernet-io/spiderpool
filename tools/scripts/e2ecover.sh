#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

CURRENT_DIR_PATH=$( dirname $0 )
CURRENT_DIR_PATH=$(cd ${CURRENT_DIR_PATH} ; pwd)
PROJECT_ROOT_PATH=$(cd ${CURRENT_DIR_PATH}/../.. ; pwd)

ALL_CASE=$( cat ${PROJECT_ROOT_PATH}/test/doc/* | grep -E -o "\|[[:space:]]*[a-zA-Z][0-9]{5}[[:space:]]*\|" | tr -d '|' | tr '\n' ' ' )
if [ -z "$ALL_CASE" ] ;then
  echo "0/0"
  echo "error, failed to find any doc case" >&2
  exit 1
fi
echo "all e2e doc case: ${ALL_CASE}" >&2

ALL_GINKGO_CASE=$( ${PROJECT_ROOT_PATH}/tools/scripts/ginkgo.sh labels -r ${PROJECT_ROOT_PATH}/test/e2e )
if [ -z "$ALL_GINKGO_CASE" ] ; then
  echo "0/0"
  echo "error, failed to find any ginkgo label" >&2
  exit 1
fi
echo "all ginkgo label: ${ALL_GINKGO_CASE}" >&2

TOTAL=0
BINGO=0
for ITEM in $ALL_CASE ; do
    ((TOTAL++))
    grep "\"${ITEM}\"" <<< "${ALL_GINKGO_CASE}" &>/dev/null && ((BINGO++))
done

E2ECOVER=$(awk 'BEGIN{printf "%.1f%%\n",('${BINGO}'/'${TOTAL}')*100}')

echo "${BINGO}/${TOTAL}|${E2ECOVER}"

exit 0
