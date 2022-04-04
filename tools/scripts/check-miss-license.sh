#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

CURRENT_DIR_PATH=$(cd `dirname $0`; pwd)
PROJECT_ROOT_PATH="${CURRENT_DIR_PATH}/../.."

FILE_LIST=$( egrep -i "Apache-2.0|Copyright" ${PROJECT_ROOT_PATH}/*  --exclude-dir=${PROJECT_ROOT_PATH}/vendor  -RHL --include=*.go --include=*.sh  )
if [ -n "$FILE_LIST" ]; then
    echo "error, found go file who missing licecse announce "
    echo "$FILE_LIST"
    exit 1
else
    echo "all code is good"
fi

