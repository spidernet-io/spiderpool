#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0


CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)
PROJECT_ROOT=$( cd ${CURRENT_DIR_PATH}/../.. && pwd )

EXCLUDE_DIR_OPT=" --exclude-dir=vendor --exclude-dir=.git --exclude-dir=api --exclude-dir=apis --exclude-dir=client"
if [ "$1"x == "check"x ] ; then
  TOTO_LIST=$( grep -E -i  "//[[:space:]]*TODO[[:space:]]*" ${PROJECT_ROOT} -Rn ${EXCLUDE_DIR_OPT}  )
  INVALIDATED_LIST=$( grep -E -i -v "//[[:space:]]*TODO[[:space:]]*\(.*\)" <<< "$TOTO_LIST" )

  #find invalidated toto
  if [ -n "$INVALIDATED_LIST" ]; then
      echo "---------------------------------------"
      echo ""
      echo "error, find invalid TODO comment:"
      echo "${INVALIDATED_LIST}"
      echo ""
      echo "---------------------------------------"
      echo "please follow the format: '// TODO (AuthorName) ..... ' "
      exit 1
  else
      exit 0
  fi

else
  NUM=$( grep -E -i  "//[[:space:]]*TODO[[:space:]]*\(.*\)" ${PROJECT_ROOT} -R ${EXCLUDE_DIR_OPT}  | wc -l )
  echo $NUM
fi
